package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body == nil {
		resp.Body = http.NoBody
	}
	return resp, nil
}

func newHTTPClient() *http.Client {
	return &http.Client{Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}}}
}

func classifyHTTPStatus(status int) int32 {
	switch {
	case status == 429:
		return ExitTransient
	case status >= 500:
		return ExitTransient
	case status >= 400:
		return ExitPermanent
	default:
		return ExitTransient
	}
}

func resolveMailgunAPIBase(region string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(region))
	if normalized == "" || normalized == "US" {
		return "https://api.mailgun.net", nil
	}
	if normalized == "EU" {
		return "https://api.eu.mailgun.net", nil
	}
	return "", fmt.Errorf("unsupported Mailgun API region: %q", region)
}

func callMailgun(ctx context.Context, apiKey, method, endpoint string, form url.Values) (int, []byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth("api", apiKey)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, respBody, nil
}

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "mailgun",
			Title:       "Mailgun Email",
			Description: "Mailgun email service integration",
			Icon:        "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAQAAAAEACAYAAABccqhmAAAc0XpUWHRSYXcgcHJvZmlsZSB0eXBlIGV4aWYAAHja3ZtZklw5ckX/sQotAbMDy8Foph1o+ToXEWSx2FXqbpO+xGRmJGN4D4C738EBuvNf/3ndf/DHQvEuF2u11+r5k3vucfBL858//f0MPr+fP/6E788/Pe9+/hp5TDymzws2vp8aPF/++MCPe4T55+dd+74S2/dCP+4cPw9Jd9bv+9dB8nz8PB/y90L9fH6pvdmvQ53fC63vG99Qvt+5/WmS79/uT08Yq7QLN0oxnhSS52dM3xGkz/fg+cDPmGr845nueMgpfi/Ggvxpen8s8K8L9JeL735f/b9b/Di+z6ff1rL+iFr96xdIjT8/n37eJv564/RzRPHPL0wL4x+m8/2+d7d7z2d2I1dWtH4z6mcevcvwxsmSp/exypfxXfjd3lfnq/nhFyHffvnJ1wo9RKJyXchhhxFuOO9xhcUQczzReIxxxfSea8lij4sYBYLDV7jRUk87NSK34nEpKWo/xxLeffu73wqNO+/AW2PgYoGP/O2X+59e/He+3L1LSxR8+7lWjCsqsxiGIqefvIuAhPuNW3kL/OPrZ9H6XwKbiGB5y9yY4PDzc4lZwh+5lV6cE+8rPH5KKDjb3wuwRNy7MBhKIAdfQyqhBm8xWgisYyNAg5HHlOMkAqGUuBlkzIlqcRZb1L35jIX33lhijXoabCIQJdVkxKanQbByLuSP5UYOjZJKLqXUYqW50suoqeZaaq1WBXLDkmUrVs2sWbfRUsuttNqstdbb6LEnMLD02q233vsY0Q1uNLjW4P2DZ2acaeZZZp022+xzLNJn5VVWXbba6mvsuNMGJnbdttvue5zgDkhx8imnHjvt9DMuuXbTzbfceu222+/4GbVvVP/h69+IWvhGLb5I6X32M2o868x+XCIITopiRsRiDkTcFAESOipmvoWcoyKnmPkeKYoSGWRRbNwOihghzCfEcsPP2P0RuX8pbq60fylu8Z9Fzil0/xeRc4TuH+P2F1Hbgrv1IvapQq2pT1Qfr582XGxDpDb+t4//ry/Ud1gl5h19OzET8LSDMqVfkBqyuCOGmU9u+xQIIIyUZmd14dq8Lc9BPIodd1a3HW3eXgLhsZJubyfdNIjgPgRrltYyuiDPQH7EdibxJBNbHnOVukc/swZ3Gdbk6UGYuTAgZvWeMHcetkJtvYYSZ/ck7LKjrI+e18uY/rbMixZrndOclWWC7NjX3fnUSW4CgEZarVtqP0XpXEmuM2rO6YS+k0AynGQ9H2Oie7XkbkZ25KvEYgLGmpBgRf9ere17GGunuiytySfy9MifPPjoPIeqAhximoNau0t56/3JueoxcN/9uW3O0Emitt5tw+qkO7+WcXMJZ8eyF7Sa/NlldheMq1Md9fpGRR2/ktXTZjs5DV8TAWgsaWEFWr2F4o3ARukdNs697Zluq/6g2JKiZwk0GGOnbHHnxUpSkr3M7W+KRuSjGFtM2M4YMHcGW+aOFKriOlpzi9nnCeUMxtgpVXhjnToYdLm8tVDr556Smx1dl4GSYQS8bWtz8/Y1FovlgiIz8+bOgOqpZAMCMGTovzExkissA7oCfHb9qBuM6JFVIfQsVS7Sozy6H7/8i49FmU5ek7i19shKdFaUdHWDZEkHTM1KoHHWjm0TtNpHA7c36nbHm42c4FMliRL6SlTM5mPCNquzpgFlz0q5BdVD2rMvAsjKlnZnBy3F0KwHnNBQu7m2pIEBsQvCHpOYhot3SMv5klaK89x4UHX5TaOtRRzK3i0Ogs8MjkRlD3vB9qUBwCcWABk9B+FUit07SKOckI06JQ2JLHcJ6YyQkh0WI/u5SLHZU7sW7Y4aqWImn9vxY3dq8YgA3Jwg9BnL6sbfdOIHcyT4kTtNSpZVRgRyeWjstk1yDNvlrnWCwdXzdH+X3YPOVvZwiVYyOm/vtedgdoyXG+e+EKAn+3U17HeFAGcRu5Xg9otQHZnpLpf3tEZIfCvz2iiRuk8NTCB9Zgh3U1yz1/JZlO5DrbHuk8Ebblby1ZokEHLxj2qBCqbiSrNBKZ9FKVPAY4YJk8bIJYY3RDVZjN+AdKmRuCFc66Xeme90LE2FpvbKNif5MxYVOPqtVEqpJEACHsps8cKxpZ3AqmkxSCLwjIo+pgzpTjHr3SfyB1WYdwWfx2RkYSXSQNkBpAIa54FNtT4++b2rimnVQhgsXlfPMRmHUt49mAZ5ivorkP8ajCnhn4hiHzN9sqygQvx4BOL/eHS/P/GKtoRJqQweG8A4fQHICA7+sVzUyEGD5ll3iYJ97op4db2j7ykPymYphaRt90QHedgIgMy8TA0GKmtVHMPgDhRTYkHXRO94imnsMhzyTyzS9+1UDXmxApEpaYtRptale80H4TNt7FFJ/InXmB6eiOnMyQjuvI74EjbUyBL2celAlEFmVruCFbtGwGtmCnbNF3zYjRsSV7CBpaYqukFmbqF+zJjGqpGIN1YDJVMOi1QICH8rs+zEQAQ71rKoJMzIoSUIzCAJjLgcKi0UZONhYWwaNEt0K5xGknXxbyEBe28hKZUeiQ4mhRC7vq/YifQg+7JLK+zWUqW2A3cEBoqQhhCcl+mLy6qyAqi1WIEGGYFK3Aaciq2S+62H7l2V0g1zEo8LD0Ppc8E0rYk5QaKxzxV/atlbtDdgMrsIBAgkLDlR/ru6iOIE2Bfimk9AFMChQA0JPWMWI22kHXXu800dNh9k1pvV7XUWZpXS6rs5GNzfqll5OxS9tGS/lkg0LgTGRVnaIDswcVJoz8C8O4qChWtv4uQLI2LmqNL6YkgQVOULdD/kzeqUzQthJ1erDWRHqBcZHEI4VBFcBb3NgfowhyA+/qktBgBEwKcGPASB6fHtrops5kNJkgiuV4z9IcbrxRhYnC9bXetK1xGUzxRjYUiJFy8DnXeWyljCgsriWeAJOo2ULcMP/DsfAs7BJPJ84bKjh9xXBYQoF6JGLaHW59jYGiBoVe4H4vA5MqTsS6LuHFV4aC80HSQ+LTkEg2Ajq/Io+L6YTTtoj8PCktWEOnBlnkQNDtaAS1yQ8oLrzBGlIlgNwSnXpfwp3Yqu+WfyVdrTD65+7Vxw5t4ao27uVAuUYtoHZoWvBjxYAQ/kGelURESJsAWpBxKH6GOLuWNmXRUTQ9WR3MWB3iMJJiE7BAaeo+B1g7BrLaLLbIBivNsH4Ig4abo0tsbi48RUQKyu68Y6qJWyIZsNRvTEelJKryiYhdcYSdJySvdHPD4TyBUrdUH2MUGkJtx/4HOKuibGi1+EBGwDMYAlpYulmkQdibR4HVdYqNaGtr7Kp00SoXLBOruGzWpRgi1QyyhIykDFyIX4JkuQN0maWWIAtQkvrlkAxis4XRL3F+U1lgiSuM90AAPK0yr36YQUQTl3AHtk9VhVL9wDL2R0kZsAufSbCYTRWhc1Qn1n0QxKGqjaqjWYKz/a96SzF+3jXOEVUoTAUIBG5kGjh/AReBiVxXIZoCAuyMuSyBHUdETDSXuVjiiXXttaRtFwNA8YLGBC1QhWAhphtozVXg6XK+MbGW0IzxFZgC8y5htHIT8CmDBdyrbmxoDAH4qPUiehkdG4B5Yf6YcbTlm5kNToKhN5qLWRMCjVUPosCdoG3jDgAhjdVD3zE0quflsHey6VAx15FDSoGiSNqUuqTlJliazCLhmOuoQh3Oe/iEXL4OfIc4sH0BBK2QPTFmlydeowByy/yH9vmCnxdiz5WtBsLVT4E2/SHygVrT8CoyyJiy2IdhiKTZmkS+klFn2gx4Ei8gclT8HfhOxCy1c0q7Qwiu/Dv7hI+0U7ud/FE7qDyfPRE7Em9xxkQ5334DOYuBARE0MFFUuiEx4oZivV7ZhD5GJEBFBCK7xqxixhYbgwgThJHBhRYIKMOSMeFP4nmQD2ZKT3BKkcYEXBs477KRsigvwcfyF2eEwKqpwq/iaFSYbIcaUY/L4OYkd+g3ZFsgZtTbqWjtyEmQEd2KNkQOlIRjzNRv7JrjLzc1hANUAG6eb6mzE1WdqT8xsIo54Ry51UPAZsom/XSM9n5oDWRmGhXqV6ekqVaoxLeeQHLoMBY/UvN51yy2hqrANFhzrDPFJENWx0tFFE5cYUMH+selUTBhG9vORxlm8l31acTA7iZP4LbuuTxWOZkdjcBDgjm1cC6PAxWAniwjsJArgB3nQ30RkALImjdENajUBFofyjKos0Jg1hM/TjFzT6sQ0+AN44GmRbOJEVyc5soogxEpIlO1ZrOQlqcSh25KYjwjwMfQy5ndBJuBuIAc72RqlRfzdwPYeUPAIgCuxm0FS1vcpWmmEnZAvzNCCYqGam2Q8JD8GahOkGAEguoIA1klJB4ky8HVpjogCEKpB0eHYIuS2tJB4beIUZYQflRpWkyXl6BBSQdI4TOkwAGALKyKxR5I4aRnSfjzsCfPFcXWoisHjYM2ToVVuPf96GEzsU+UYeL5+ldEgG8O6ZGVZpsf4tYJjK4wt8CLQi95ETpBGPX9T4QQ9bt0BdJicywB8Au/Mj88AjjyhLgSAN2BKBC0DOm0mmlFkJw1aieURXAoSgaKJqKRAylQoHPchvcgx2Qyp5CAQ+ZQIMDxRrP1JMnUdMzgj4sSI9xks4Iy40qgYR02uxnHEkJEmaeKoyDYdUkZC8txrqyUC4AtuUQ4bulNE3uXe8qlPXi2pG4yeBH/nNJEkPv4BknGQNOLSDCc5IR+gfvdE9ymEAk2EwMtwuCr0j2I00LJOCQmcxSWlJFRS0Ly7roAbI9ja1hLkF9PJTLE6p5/ATNBySuijWu1afZSdrjkKNhqa7p0UxHIUH9QGYBxiDuWWC8ge+IJuehVfuLwFsNqbGLAMrQbpFw8tAV+iYQ7A3RabaxJxyr+wlKq4a41HLTNYhU4p6IEAN82VmeDoZWfIZ5RexOUO2YjZgBUTwLLS6AXLsakQxRcyhvAGBhKmgFHQSgAZ1c3PABX+Gj3s9FOQjM2mrB7Ifcr7kBcwGxro6EedTSubEgGlTKwTJRfjRhKhXSFqjAGEwXPhLAJEyamg+72sFc2HS4FNysTJTRDzULdNGwSKSeA/mDFSGKfcSLYm9mTPuLjV1uBuIRjlk0CB1cHFiagC1ZmoUZLl1KGQvsBDR2CMphCTyXffPh1l6cbfEvSwLaItq3Z6MisVFEEw5F4+QqO6lnT2DHIzChQEp7IeLV615LHE6O/nXTMB1tRx4G7R0KiLifK95YG04BAWEGyAH0VnejAz4XHK+S0IRuAk8OazFxUxbAZs3ePdUgrr7zBopXVlbNB6mYkbJW+JSpLSw+fAFrDDwcI1cJ/pQu0+gY9WUHXo6ICYZEG4Ufo8vUax9E6UNyCZpiX6EyMPQiWHWJUkXVsI6c3enzWqqDld9qT55W+wLIFtxWXyA9VQZBy8rABlF7bOQjwkTNMhysZpRoMuxjkpJeylpIOk+GIkTkGrA1cYgQQtN4Ux4m004k4QiSxB+pJPyMCohRY0kJPp3BFPvBcT+ZCPrtDLgtABGPBMIvjIKKgWMyzJZQWgG9R/jdbd7Lv3azvAUig0RsKi2bMDSJOE0tsMbCMG34jfWMLbfjJP76aDQFishxiJL9hrSLQhHyIPyRoa/jG9ktfmgka1GujEyvbcEp8FBwpRlRxUgmqsGEepZU+B2k9c0eYRBVNeHaiEZu7YhqSjfz6T8Q3ET5h/iT1Zmw5tkF9P7flgXqcKzZKh5ZBb4m8gm6hY5hCNR2vVzqXGHuxOOYsJAe1aQAi1kOAjc4DU10yblN9RdU3+UPELZqxlZ5IxVlxOJN7tTYRaKpjFEhA/rDrICrahQcAGuw9sh2vezbFwgj5fjnjXyL8cxNR5NRUIip8AF+1RIfmZCFSKqkGfXTtxp9TToGV0Sb8pKOyVZbei4jGhhpm6fBFYVTxaBKkC+P8wP+8gqkqjEAgsWtFsi+K0CEVbOl4QHATdfj5tLTozfuyYwvLUhJ5/PepGYYktR8ueSwCs8t7tuqEth2xI8fkUMBjIO91Dsgh0fFOvfElXXf3xgwT5gSGHE8MCQd0z11Qp+nww7gHtfSD/GMkGqXcA7HAfiveQTQFzkqFeX6RMi+a6plgjDUa3pZMJLIiIUizmgz6xukvFK+wCfeOTdtb0qQfPcVMMtQ7uTGde3L/EEZkY61gO0LnlgBxc+IZ5Ix0YykX9pMUxwR7pFgEl1IbLRHMhg8KNXOVWPAGNFP51OhIiDuCl6pgawib7RGQhvfBLwlFarS4yJj9qvV0tktnbDkHJyL2uYB6WomOwG0h0foyx+cPAXaDBIPF6Dr5byYeEffOggKNcN2hFGdncHySQuhug3+aA8AGzSEx1Ddr0s4SY31L/bLDF55PA2fOVf0CpSIsX8cziRhESJfNxLkCWCTSRugvyWxA1zjxtP9RwRc9hOHauCngJEQN1C5JEusWxEPAhtK9+qMBKq2UgOagCfvxfuWo0zO116d87qxirqNKmAUXJrDf+U3GzYgyfkTHsoK2oXyeLrlQxxFHBC8JGBhyEJj8qWawAeoW4gnpVHNJJ5Bps1MdfNT3Jh5SkRhIr6ImdFQRFqV8eakCjpOoz7bM/nBKlPRL5sDuQIHJb0bM5GwwxoLEbQJOYpC50kayB2RR8mS6jajdDDnHkpgHa2hNFBfHrtBNhSnzOqDUBlT1U7bu6x18cYsXgTGwcMORIlMgB4XdshGKPXgIIgj7Zzd3w4VrOpg3wKF06KDG4c79XUy6J0BGdOTXlRu1pZXWdxnu+IVPzzfTiXpY2I51wWxu9SU+p5k2DyO0t+R4fVnKgc6Yo6PAhzr23lIMr3pAc4gyO4knntmaJnKbu8YGawAL8Wi4lhc9xRe7qlffAFKL+BJr9wHKvF05XLF3UddO5gZOBbq3XVROiBz923SNnAaicLJvcoLZx+dY8e5amNozImDhgb9Aq+w0t4FzW8qJ0v3YJ6z2X/3i1AhSxgmgug6Mm6DpBU8phyvOgdpEqv3g6sgeAiswpJhhj10CJaqaFRtMlql6ECmR3AxdCs8+0YpvB2/nLHNKRHoXD4SiBHYGK2XctSdfmrD7THfv6uoxEI3VSDBhFtWnW1VEz9kh2jTmkEwH1LTyedBAk5+afBSbopovl0Z9U7Pqtqw7se1MRdElgguJqzAJMDJKqkYpmo7GxqvBDA13fpW+0aUlJd2fLaQ9oQ+rSHtLsCv/djXYSenHqXkHiCmoS266iBBm93bDXsAXgObWAC/9r7BxOmNpKYZAhbZTDDpxXr1Itt2pcvhxyvapHpcMFrkYGxwBNsYtqADy1hkSkKU+3r45J4ap7Kkbuk5rS6p6buKely3tY4wg7IMvH5kzdQf1aLnMW+OkBASe8kDX1AQZLkOBlyfUOTmH4zNDuQpQ5/Tb49m33x929XGpTGDCFBWAKikrR6lBuCxV93UoaQtd0XHp7oXVHtTg/x5tftBFJ2Dp8ewpseepwyBP981f4rYOmDxCh5/FqqGaqrykFUcpcYZNXuUCdUjTT1aVsFs9SnhSe2+rQNDzauoukSykdb3tBhiVR7lBVVUyR3ihxQopRj0JV5MkZkd3wJCnPGpaYyg4JVg4OYKkrDy3yr2SoDHva3e1ows1nyCm25MM0kIdDzgvFtZWd1ZfNAsN/DTC7XfW3Z2ZY2dZ4TUGt23qT74fmx8+dHn5VVZ7igZ7z4e0DgCGpVdWfLQmK8AR4EOsRdAtAXcV9omWYiD7IzFtydztICT8eAPFZXblrHWdwIQcZkZ0tVEoFocO+L1ICH+iz40Iox1oGAJC08vc4FoKwgmX2szRgLhn05leyWAVnaMttqrhDe8fb2cKENAaylYqIJmbvVU2QgQFVKGQX0JBz4sl3W9m4BHlFNW3sfH8wIb09V54H/4TF9+s6/tZ1hkaiNLYB5o4QJLhWC+IV2UNuG4dz4bdgaiiECLBlCIwyosYEQGziOfi4KxQXlLhSdH+Gegy5E65NaFYH7tsp1rgI9VbRLor0oEhATHN4O1ZRuuNrTcVF7+nOHIHUJ3ZCJsL4PrPuV4r7gVoWN1FlYDCEI+RsWL4cV68zklrRbdq3mtxkAUKS3GRCCNgO0l0+VaOuWtIW/k07rqRPXEBhYWp2gYMb3za/LQW6dEMKewlSmfbZZXzKinXC1gzTmr3acFiWtji1Ecu3ROvLP0Jtr4wiHu2p3DHIMbr5rbKUX4x/KEIqVaWYPhhxfZZSwOeRs8e8MlzKElE9N2eAiKpmJaDcLXGRVRspwNLnMpYPnOjypFc/gGOtFiqOSHtjpBBZZr9ug/HXYSXXUdATlnZuV51P3StvrqCIEBMIfEsDfF+JszDurm68zS55En6xsrw6EmibC1ykSEIzEv5oVbloHLXaUCr3cLn8s/qs/lSNR/uyai0iwWQBjRyW8bfMG9SjNQR+Almyv8kLU2zydIrHqh6EXYVuEr/a2tHOW3y6gsdh8uIYXRaR33VzZa/+HGmJc9gmi9lVZiT7I3dNry8uoMtgpDCBsPlnDMKog3n+3hnVqLnhtHmknpb8Bbk+6q0k+EQTjghDq3MXwLJ1OflRYhEq6MDE+uiozMFCYzXc4jEVvARO9NQqJTuKEEi8dwE2kPiKCUkY3Xz7lqA+cv5o/lBoCNRf1mrmupOfQCTB0fqRQ59uUfvpSrYIilR60xaQDJvBalL+R5USxfKBCm3/tb0Hk98c6T5tjB7eZD0okeNzh1CGhud+csLdDB7fi6xrp4JYoq2ORsIWsefwNJ9wPoNCxKw8C4aJ0KKfrTJ8O5bztYOHyO5QDAm7QgSQRGgBGVcdSDKRzTXgFhj9+POqoIeVQ7+R3EPLn+bb1q44dlLL9R/PrWBuCTkRXNfhzHAh1IVId1gXrctQGAt6EfF+UPcFdmN9mlvDLDIZ3g1ZvT+4UDDekc57Ed1mnFAAXDAE011gCJNdm9XX0Vv4I6mo3aTchnspFkI1MCRmQqSjjzh8l7l57talIKYlsADA5KAnz42SGfzbh7XhoH/XnjkcE7/pzCUjI2tz4XAEhBuQxhKu2Y6CQps4H1M8xNHSXjqGZmAsHUpVLeFhx4FuVqj5kXvIiDQEa3/64lkUHvArTNW0fdmBRpxuZbtRJYzum6eoUdM6+if5Kru77/y4QLfEtP0XcwIXzDuYdHOp6B/PeWTSEHZD3jvR5uZ6sI33QPb6hOGylzOtUwzcK6HHpfJakeAeEsiBbfSNqTZsMzbyaujyPytV6KTk6y+N4CzmmzZNGklB4GC0Atjwvtj5WY7elLhJiBGs71P3Me2GmeOFTKqgz92+fDv6bsyruHVb5lbSZDKEbMl0Gd99pHSjSGWCUDmsnpNWZRyyVzopS6nmQEA5ZrrMhRWdFjw6xIVfx+xQJ2ja1J1qR+pV6kQCqKO3d0L64jiSpdbQbwRUcYzFZM8UgD6/CLWoTzIabLDo00Kmr/E7q6pTwFLlEkTUyRg2mQA2tu50/oqdMbr4DwvgtfrKM5x3AKu+UsHpBMedYSH0d14UPNuoZeEMk3ddFhftnfjuwn/9e0HQAIxR5syPuWQUBNKY2x6cOHnHlz3FiRou81zac9lhSvWIRnRF+N036DwP5HU0O+h8iTTfV0YqJMsM0NgqCjx81sAdLKhN66tWEZGq4HniiM8LqxuqMsKfA3xlh7CJiLKo9A/QN7GjDhKPkdfBbBzaL9HzV6WB1a/Q/cLo8k84YRoW1CP/SGu9oNzU9dVj/xwng+KZJkhorWbWDOLc3d3Rklo/gnog76IZ6/eXsG5pWp4MlSsU1Oh1cicqoXQJ4qNWBYeGuLrfnAUS+sgCQLzBVtRfyORvM5+u/cgTY5X/zzPD/swshGe/uMMV/A/5g6uorn3rlAAABhGlDQ1BJQ0MgcHJvZmlsZQAAKJF9kT1Iw0AcxV9bpaIVQYtIcchQnSyIijhqFYpQIdQKrTqYXPoFTRqSFBdHwbXg4Mdi1cHFWVcHV0EQ/ABxcnRSdJES/5cUWsR4cNyPd/ced+8Af73MVLNjHFA1y0gl4kImuyoEX9GDCPoxgEGJmfqcKCbhOb7u4ePrXYxneZ/7c/QqOZMBPoF4lumGRbxBPL1p6Zz3icOsKCnE58RjBl2Q+JHrsstvnAsO+3lm2Ein5onDxEKhjeU2ZkVDJZ4ijiqqRvn+jMsK5y3OarnKmvfkLwzltJVlrtMcRgKLWIIIATKqKKEMCzFaNVJMpGg/7uGPOH6RXDK5SmDkWEAFKiTHD/4Hv7s185MTblIoDnS+2PbHCBDcBRo12/4+tu3GCRB4Bq60lr9SB2Y+Sa+1tOgR0LcNXFy3NHkPuNwBhp50yZAcKUDTn88D72f0TVlg4BboXnN7a+7j9AFIU1fJG+DgEBgtUPa6x7u72nv790yzvx9aYXKdxZa9CAAAAAZiS0dEAP8A/wD/oL2nkwAAAAlwSFlzAAALEwAACxMBAJqcGAAAAAd0SU1FB+QLGhITLEXgL7EAAB84SURBVHja7Z15kF1VtYe/2525k4AyPYooSLi85CoQFBlEMAw+eIKJB4QQZagAkRQCKoOiIJOgSBgUlAckkDKiENFcw6AoUyrIKMoQOYmcICAglYTBJN10Okn3fX/s3dKEbtKdXvvcc8/9fVW3Gi3YZ5+111pn7WGtDUIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhHgvBYkgX5SLpQZg/d84oNH/HeD/1Qqwvf/nf3TRhXXAU0C7/9vR9RclcYekLAcgsmPo2wEHA7sBuwAjgOFAEzDUG35/aAdagRagGVgFPA08AdwNvCjHIAcgwhp75ziNAw4HDgI2B7byRl5NWoGlwOvAncDtPnIgSuKKRk8OQGy84e8DTPFf99HAsBrp+tvA8z5KmBUl8YMaTTkAsWGDHwJMBSYBOwEjc/JqK4GFwBxgRpTEqzXacgDCGf0w4BTgCP+lrweeAG4DfhIl8dvSAjmAepzTHwUcDxxY5+K4F7gJuFVrBnIAeTf8bYCzgWOATSSRd7EC+DlwaZTEr0occgB5Mvz9gdOBQySNXnEXcGWUxPdLFHIAtWz4EXABsLOksVE8A1wQJXFZopADqCXDnwj8ABgraZiwCPh2lMTzJAo5gCwb/gHAtcCOkkYQngNOjpL4PolCDiBLhv9R4AbgU5JGKjwMfCVK4mclCjmAahr+ZsBFwMmSRlW4FjgvSuI3JAo5gLSN/1DgVlzSjageLcBRURLfKVHIAaRh+KOAO3CJOSI7PAV8PkriVyQKOYAQhl8AjgWuBwZLIpmkDTgJmK1ThXIAlsa/OS7NdU/JLPNUgEeBCVESvy5xyAH096u/B3APrsiGqB2agc8Cjyka6JkGiaBH428ErgIekvHXJMP92F3lx1IoAui18W8KLMDl5Nci6/yv3f99GlegYynwFpB0M/YVoAh8AFdpaDSuxNgAXFmxAbxTT7DWWAjsGyXxv6XdcgAbMv6xwJ+AD9bIfHctrizXA7gkmgW4rbEWYHV/i2/4IiVDcNudTcC+uKSm/XDlyAbWiB69CXw6SuJF0nI5gJ6UfSJwC9Wvs/d+Bt+OK8Q5A7cwuRx4M+15rl8f+SCwBTABV8loOx8tZFWvWoHJyimQA+hOoc8FvpdRo28DZgM3AkmUxG9lVIYf8NOIE3BbpoMzqmPfjZL4Ymm9HECn4s4ATsxYt1bj8gtuAhZHSdxWYzIdDIzBVT06LYNdnBkl8VQ5ABn/r3D1+LLCAuBC4JEoiVtzIuOhwF7A+X4NISvcFiXxkXIA9Wn4DcB8YJ+MhPnT/Vcpybnciz7aOisj+vcgML5eLzYp1LHxP0j103ebgW8BN0dJvLLOxmAk7s6Di6n+OYuHgX3q0QkUZPxVYSVwLnB9lMRr6nwKNgh3fn861c2xqEsnUI8OYEEVw/524BLgsiiJWxBdx6UJ+CZwDv2/z3CjpwNREu8rB5BfJavmgt884KQoiZfK3N93jLbCZVxOrFIX6mphsFBHilWtrb6XgGlREt8t8+7TeB0MXAdsW4XH180WYUOdKNO5VTD+CnA1sL2Mv+94mW3vZZh2Nt+JXmcUAeTA+CcCv035sa8Ch0VJ/LhM2WQMdwfmAtuk/Ogv5P3YcCHnijMW+Avpnu2fCxxRr/vKAceyAXeh6GEpPrYV+ESeE4gKOVaYTXEpsGll9a3G1ayfJXMNOq5TcNWAh6T0yDeB0XlNJS7kVEkagSdJL59/GbBflMSxTDSV8S3h0p+3TOmRC4FdoyRuz5ssG3KoHAXgihSNfxFQlPGnh5d10cs+DXYCrvC6pQgg4w5gT1wpqDScWxmYFCXx2hqQS2dhjwb/t7uKQKuBDgwKiaT0TgOBOUCUwuM6gL2jJH5UDiC7CrE58ALpnC2/BJdXXsnQ+xf8uzcBn8RVMd7d/xp5p1hHTyft2nmn6Eg78Lj/PQr8GVdlqDmD73wF8I0UHtcMfCRP1YYLOTL+gv/y7xX4URXccdVLq20I/p2H+rnwNFxyTRPuTL11/b51uMIkLcAs3CGdZUBrRuRwtnfKoXX6ER8JVOQAsuUAjvOKGfKdOnDFLa6tlgJ4ZR+CK1d+Ge4m4pFVGMsKLqnpOdwZ/sf81KGacvkW8P3AsqgAU6Ik/pkcQHaMfxSwhLDZZBX/lZleDSUvF0sDgBJwuTf+kRkbhpXeCZwGLImSeF2OnUAbsEMeriHLyy7AHYRPJT2nGsZfLpYay8XSVFwh0Kdxl12MzOAYjPR9WwS8WC6WpqZdj9+PzQ/9WIVksNc5RQAZ+PofmsJgXAWckabx+5NvZwFn4Crv1iLLcQt009M8GZniwuDna/1W4kKNG/9muGy7kFd0l4HD0zJ+r7xnesPfKicR2lJvkJenLMffEHaLsAXYNkriNzQFqA4XBTb+Rbh9/rSU9gDcotplOTJ+/LtcBjzn3zGt6cAkwh4WavI6qAigCl//jwJ/C/iIZbgTfitTeJfhuAMtn6M+mAccHSVxcwqyHYm7Ci3kseGPRUn8rCKAdLkhYNttuLP9aRj/JOCVOjJ+cNV+XvHvHjoSWIm7xmx1jeqiIoAeQuV7Az7imCiJbw78DgNwN/0cS30zGzgh9LahzyK8KeAjDoyS+D5FAOlwbcC256Zg/B/CZZjVu/HjZbDQyyRkJDCLsIVhrq1F4decA/AVfnYM1PyrBC4aWi6WxuMOLY2R7f+HMcASL5uQHO7HOAQ7et2UAwjMDwK1W8GV8eoIaPwn4PLYB8nm38Mg4AEvo1BRQAeuolClxnRTDsAbUASMDdT8NSFr+JWLpQuBmbLzDTLTyyqUE3gcuCZQ82O9jsoBBOKCQO2+RMBTY+Vi6QLgPNl2rznPyywU3/BjXks6Wt8OoFws7Q/sHKj5aaFCf6/I58um+8z5oZyAH+tpgfq9s9dVOQBjTg/U7rxQdft9KCvj758TuDCQE7gbdyApBF+TA7A1pG2AQwI03Y67mDJEn09Q2G82HQi1MHiS1wFrJnidlQMw4uxA7V4S4q4+v52VtQW/CrACl5izxP+e9r/O/73U/ztZq3YzM8QWoR/7y2tMZ03J/ElAn9X1FrCJcdNtwGbWt/T6Ay1LqP5W3ypcOu7vfKj7CLAWV9Woo8tcuDP1uPOD0AAMxJVWmwyMx6Ujj6jy+6zBFeF42Xi8moA3sK8nsQL4QNZLh9WCA5gM/DJA06dFSXyNcV8H4E74jami0T/pvz5PAW39Xdz0zmEwMA64FNi1is5gMbCT9bHhcrF0Ku4OQmu+FCXxLVm2rwFkn+MDtNmMu4LamhurYPwVXObi6bgFTdOIxjuQVh9BfMZ/MScCV+Iy7NL8iIzxMj7OuN3rcWXEhgfQ3Uw7gExHAOViaRiu6II1X4+S+MfGfZ0E3Jqy4b+CO0v/cJTEa1Iem0HAp3DJPKNS1qWjoiSeY/w+XwN+FKCvTVESv51VG8v6IuApgQxnlrHyDA8UUfREC/BF3J1189M2fh8ZrImSeD4w2velJcXHX+9lbskswix+npJlA8u6AwiRmDM9QJ7/HOwXKXtyXnOAraMknpuFG4miJF4bJfFcYGvftzQWvTYBbjZ+j5XA9BrR4fw7AH+V1W4Bmp5p3M8DSKeYxxrgYGBylMSrsjZevk+TfR/TiEgmBigvFmLrdjevy3IAfWRqgDYXREmcGBp/AXdDTmj+Dnw4SuI/ZnlbKUriSpTEf8Rd3Lk4hUdeZ3lhp9eNBTWiy7l3ACHKRVkfKz0T2CGwHO7FXU29lBohSuJ/Ah8nbNUmvOzPNG7zkhrR5dw7gBDXez9i+PVvwJXuDsks4KAoiVupMXyfD8J4wbUbzuhykMmCB2tEl/PrAMrF0j7Y335ztbEhnUXY0t2zgKlpXqgRwAl0+PA3pBPYyo+FpeOyPhQ00uu0HEAvmWLcXgXDgpD+yquQX/97gROjJG6nxvHvcGLg6cAZxteQ3YT9bsYUOYDeGVcB+9X/NmwXpY4n3HVdi4EJtfzl7yESmEC4hcEtsD0xutjrjCW7WS5Y5j0CGG3c3uwoiU0G1J/3D5Xjv6ZW5/x9WBMItUV4vh8bi7624U44Zlmnc+sAxgHDjMP/Gw3b2wEIketdwV02+U9yin+3zxPmsNDW2O7I3Gjcz2Fet+UANsAE4/bacVdDWU1Prg703r8C7iH/3OPfNYQuX20YZifYFwuZIAewYQ41bu/FKInfMmprCLBHgHduwa34V/Ju/f4dpxImd2APP0YW/XwLeDHjup0vB+D3czc3bnaGsYJZb09WgGOzeLw3oBNYhctitHZ4I40d9Azj/m1ufGah32StHkADtnvrFeB2w/D/sgDv/ApwR8qOdmtcQk1nYY9VwIooiV9LsRt3+He3vhLssnKxtIdRNHU7rgiK1bRiS6/jHXIA3bMdMNSwvbW4slgWDMX+SrLOr//awAY/BHca7STgy7xT/qrQpR+Ui6U24Be41OaFURIHu1E3SuK15WLpWOB+bGsJ7OjHyiIHf7nXIavybsO8ji+RA+iefY3bawXeNPTe1uH/MuDhgIY/CHcO/ae4ajc9GVqhyxrHCbg99eZysfRVYE7AegMPexlYRn0j/VhZzN/f9Do0yFjHM+MAGnLuAB4wXFibhn3Vm9NDGVe5WCr6EHu2D/X70veC/29mA6/4tkJEAWuwv++hgNGlH153Hsi4jufKAexi3N5dhvN/66OcqwhwMUW5WCqUi6WjgRib04pbAHG5WDo60Em2eV4Wlkwx7Otdxn3bWQ6ge8VtwL7arFVu93CgybhvTwYoSV7wX9TZxtO7Ab7N062dgJfBk8aybcKuwKd1fYARWdoJyFIE0GBsZOuw22tuwr5ufIiLI76MK2sV4ktd8G1/OUDb1rIYbKhLLV6XrBieJbvLmgMYbuwArM7Uf9L4i7oKV7ffes4/i7DVeQvArABrAk8ZTwMGACWjtlrlANLri+UWYDs2W0EAexq/63IMs838av9DpLOrMwB4yD/Tijbstms72c+onbexPRI8VA6g575Y5nSvM9zH3t34XX9nnO47iXDpyd2xBYZlrrws5hv3cU+jvq02jgAa5QC6xzpT6mnDtqwdgNnqvz/k89MqjNdPjavdWt+gs3tGdSmErufCATQat/eaoYFZ9q2CYW1C3Am/4VUYr+HY1rp7BNvcgEZDB/VaxnVdEUA3vGDUjrUDWIk7XmrF16nOFW8F3NFiK9Z62Vga2ZCM6ZIigPfBegHrLUMZWRrYamyTQQ6r4phZbgl2eNlYOqgGwzHLsq7nwgFYY3UBiHUEsMrKAfisvsFVlPFg3wcrB2C5FWgZATyTVyPJswMoZKydEGyiPtS9DuTGAeS+Go6nxXALcESVlbMzaajfeJm0IOrWAWyp4RCifh3A8jqReZNhMsiqKkdOFat5u5dJU1bHTA6g9qhkrJ0QrFAf3nfcrKZaJTmA2sMqYWU1tmfBR1jJ3dfwa6uijNsM6whap4N3GMrmY3IA4Vln3N4HDBXJMgoYYiz3X1RxzH5hrIuWR4stIwDrKj5/lgN4L08Zt2dVZ846AhgJDDRs7/oqTVMq/tlWDMS25mI7Bgd4ysVSE/YHd1rlALofMEtM7mLz2WCWfSsAexm2txBorsJ4NftnW7EXtluaVtmgH8T+sFUsBxA+ArCsL/i4cd8mWzXklfyrVRivrxqXDZ9srdvlYski0joc25OgLWToXoAsOYAO4y/tAMNsMGsHMN64Ltwc0t1GXe6faYKXxXjjPg6kn9t3vv6hdbmyVQGi3dw4AMu5USN2tww/avyuW1iGlb689t7YL6R2G1oDexuXMx+MfUGTwbgbg/vDfwfol1kuSB4dgOVcdgB2Jcb+bGxcIzBOCY2SOMGVLg+5IFgBpvhnWTIO+4rQBeDafrYxI4CNJMbVoHLlAFqMHYBlZVjr/fZLA8jwF8BZgZxAxbcdYtvx0kA6tW+5WNqoQzzlYukzPqqy5ncZsrnsOADvFa0viLDav23GPlFlV7/FZCnDCnAl7uZdy4hlnW/zSusrzL0Mdg2o3w+Vi6Ut+9inrYE7CZNodZscQM9Y1147xNCwZhn3bQQwMYAjrURJfDPu+KrFwuByoBQl8c3Wxu+ZGCD878qmwMJysTS6N5ealIul0bjtzRBl1pYBb8gB9Iz1LSz7Gd5kc12A0PpK4/La668JjPJf7r4mDXUm+RwLjAow5+80tkE+YgnNlri991+Xi6X/KhdLDV31wl+n1lQulm4E/gZsFqgf/yRDC4Cd8+Q8O4ChuIMcFl53Ga5m3SbGivkp7EtidzqBNcDPy8XSbbxzPfikLmsj77oe3E9z5pDC9eCeT5FeGvggXPm0w4CXgOXlYqkVt1u0GbA9tic0u+OmQFHURpOpSiflYmkALrvMavuu4sPXxQZ9KwCP4W4JsuRlYHSUxGtTlPPW3pF1ht6rgBWGiT296cNA4HngQ9QH7cCmURI3Z6lTWYsAOvyXdjtDBzcB6LcDiJK4Ui6Wvon9ddGjgM8Dc9MSsjf016o81hP8u9cLz5LBikeZWgPwOwHWJ9qmGrb1GLalqzud1M/LxdIm9WIJ/l1nk+Nae90wI2vhf+YcgMf6PvbtysWSVWrwau8ErBkGXG999XZGjb/g1xiG1ZHxrwT+L4sdy6IDuN24vUaMioN4D34aYVZyjwQ+WwfG8Fn/rvXE7VESt8sB9I6nsLvVtzPEPsGwvSWB5s8F4I5ysfThHH/9PwzcUWehfwX4ZlY7l9WSYM8bt3dsuVgySb6JkngdcGGg9x4E/KFcLA3NofEPBf7g37GemJvm7krNOwAfZj9h3OxgYIxhezcRLv12DHC7cbpwtY2/wU/txtSZ8a8FpmW5g1lVMutjtwXgeEMn1Q5cEfD9DwRmloulxhwYfyMw079TvXF5lMSvZ7mDhQwrzgpsa8QBDIuSuNWofw3Av7CrPdiTIzwxS+mjGyGjmbg05Xrj71ESZz7iyXKYuTBAm/sYRgEdgaMAvOHU5JpAlzl/PRp/B/YlzurOAcwJ0OY51iEeblcgJAcCf62l3QHf17/WadgPcGqUxE/WQkezPAUYQpjyyTtaZreVi6UDgHtTEMka3JHhe7J4oszLooDb57+D+lvt7+SGKIlPqpXOZjYC8JloTwRo+kTjft4HzEtBJIOAu4Fbsnhs2PfpFt/HNIz/MQzq/htzdy0Zf9anABCmespZ5WLJenHxaNK5I6+AS+f9V7lYOsyo7HV/DX9guVg6GrcgOimlqHKFn14cRnbubrwb+FythStZdwA/CWREpgtTPsUzTc8/DPg18Hy5WBofqqjIBgx/ULlYGo87tDWbdM/2nxQlcXOUxL/Hfl2nr1SAHwOHZHVqVpNrAF0U7R7sF5Oagc2MS1tTLpZ+hquik7YCLgNOB+ZFSdwSeDyagC8B38MV80hbh2ZHSXzceusOFwLnVqEvbcARwF21ulVbCw5gMvDLAE2fFiXxNcZ9HYDbvqzW/u8q4EncZRZP4W7v7ejnOzXgTlKOw1Xv3ZWwNfzej8XATv44Nus5gcm4cxNpREMVvwZxRJTEr1DD1IIDKABvYVuKq9N7b2b9xSwXSx/CbQ1WexV8Fe648nzc4twjuKOpHbyTzVhZTw8a/G8g8Glc1t543OUYI6r8PmuAHaIkfvl9ZL898Htc9mchkOEv92sdC2r1q19TDsAP7DXAKQGa/mGUxGcH6O947CsHWSjvStzKeWf59c5t1s6DRiNwV3SPzKBu7Bcl8fxeRmEHATcaTlE6fGR3IvB0muXb5ADcoG4DhAi12oFtoiReGqDPJ+COwYr+c3yUxLP6KP9GYGfcxalf8o6tt/reGRk9hCvk8acoif+ZR8HWTF52uViah6sjZ828KIm/EKjPFwLnyX77xUVREp/fz3EYiCs++iG/PrM97743sAAsApYCf8fVe3g1harIcgB9GMT9gfsCNf+/URLfHajfFwDny443igujJL5AYpAD6DSmp31YZ81LwPahFnXkBGT8WaXWik6EUohtgatCddor8kVStz6F/TJ+RQDdfk1jYGyApivAnlESPx6w71oY3DB9XvAT9RMBAHw7oDOcG7IUV5TENwL74fa0xbtZg9vqk/HLAbyvEc0DngvU/DbAbwL3fz6wAwa3FeWIxbhDPvMlCjmA3nBywLa/UC6WpgV2Ai/jLuucLRVkNu5478sShdYA+jKffgh3u2wIVgOfiJI4TuE9JuFuytmkznRvBS6rb47MUBHAxvCVgG0PAR4IUDegu2hgDu6SzHl1pHfzgFEyfkUA/f16/jTwdGARsEtaZ799ebHr/BpBHlkCTPNVlIQigH5zHmGvXB4LzEnr0k5vGDvirpJamiM9W+rfaUcZvyIA66/mobgilCG5CjgjzYovfjvyLOAMXDpuLbIcVzp9epqps95h7w/Mz+qlnHIAtgP+JK5gRUi+A1yadtknn9V2PO4o8dY1ELV14JJpzgFuTtsAvfGfDVyCK9pxUJTEK2Xq+XYAo/z8cnDAx1S8E/hhNWq/+Tz3HYCrgT2wvzWpv6z0BncasGT9qj0pGv+3gO930e1lwMejJH5V5p5TB+AH/zhcSahCXp1AFyUf4p3AZX7NoBoFPDoLjDzn5/ePAaurLJf1jb+TVuDTURL/VSafXwdQwBVw2CsFxT+nGtOBHt55KK7yzTRcteMmHwkNMH7cOlwZtRbvaK/zX9fWjMihM+zvSafXAscBt9Zi9V45gN4pwubAC8DwFB6X+sJgLw1huHcCnwT2BHb3v0b/K/i/3dHuHVy7/z3uf/cDz3rjb87gO18BfKOXzvtHWRs3OQBbhdjTRwJpLJaVgUm1UCPOX7U2xMulu/JYFdwJyA4fyq+ugXcaiLtDMurjf3on8MUoidvkAPLnAAr+6/y1lB65CJdGrJXmdMd5JPAoG58a/hLuuPcbcgD5U45GXH38nVJ65DJcKmss00xlfEu4qstb9rOpfwP7Rkm8sF5l2ZDHl/J7z/sCb6b0yC2Bv4TOIhRQLpamAH8xMH6ATYHHy8XSMYoA8qksY72yDE3xsb8FDs/DpREZG8sGXK2GEBWcK7hdne/IAeRPcSZ6o0yTV4HDQpYXqzPj3x2YiyvYEvRRURIfJgeQPwU6F3eZZZpUgGuAbyga6NdX/yrg1BR19Ungf6Ikfl1rAPlZE7iY9ItxFnDHYv9RLpYOljn32fgPBv7hZZjmh2pX4LuKAPKpVL/CXedcDebhKuAslXm/7xhthauQNLFKXbgtSuIj5QDyq2ALgH2q9Ph24HLge9a3EudgXJr8l/dMej6pGJr5URLvpzWA/M8rHyRcPcHe0IbL9b8+SuI1dW74g4CTgOmEzebcEA8D+9Tbek2hTpUuC04AoBk4F5hVbycJ/Um+KcDFpJO7IeOXA3iPE5hfxenA+lwO3BAlcZJzuRdxBV3PzEiX5gMH1OtOTd06gC4KWc2Fwe5YgEtrfTBK4tacyHiod7Tn4E5oZoW6WvCTA+hZQWcAJ2awa1cDNwGLay1zrVwsDQbG4MqZnZbBLs6Mknhqveu+HMA7CluNw0K9oYJbNJwN3Ag8n9UMtnKxtBkwGjgBOBa3qJdFHftmlMTTpfVyAOsr8ETgFtLNHeirM2gHXgRmALfjKu++WYVipQXgg7iKxROAqcB2vFN0JIu0ApP9/ZJCDqBbxR4L/Mkrd9ap4EpdteIq9/zKryG0+P/v7f4W9vCFRIZ5p9jk5/BH4qoMDQUG1ogevYmrC7hIWi4HsCGl39Qb0k41+grr/K/d/30aV6r7BVzVn2foviLQzrhqQR/BlSDfBVdbsNH/HVCj8liIy/v/t7RbDqC3TqARV2vuVOokZyKHdOASss7QBSFyABs7z90DuIfqH1YRfaMZ+CzwmAqAygH01xFsjltw21MyyzwVXK3ACfWS0tsfFNr2Aq9Ie+OOrrZJIpmlzY/R3jJ+RQChooFRuMtIx0kameJhXIn2VyQKOYA0HMGhwK24rTFRPVqAo6IkvlOi0BQgzWnBncC2wLWSRtW4AthWxq8IoNrRwEeBG6h+enE9hftfiZL4WYlCDiBLjuAAHxHsKGkE4Tng5CiJ75Mo5ACy7AgmAj9g46+tEu9mEfBtneGXA6g1RxABF+CO2Iq+8wxwQZTEZYlCDqCWHcH+wOnAIZJGr7gd+HGUxPdLFHIAeXIE2wBnA8cAm0gi72IF8HPcFV2vShxyAHl2BAXgKFy1nAPrXBz34qoe3aoz+3IA9egMhgGn4OoS7lYnr/0EvsJRlMRvSwvkAAT/Kb4xFZiEq0UwMievthKXkz8HmNHfIiVCDqBeHMI+uOSW3XC19obVSNffBp73X/pZURI/qNGUAxD9WzMAl4A0ATgU2BzYiurXL3wbWIarTXgXbgX/KQDN6eUARDin0IDL5dgOONhHCbsAI3AJSsO9c+jvPXvtuPqCzbjEm1W4EmML/O9FoENXoMsBiGw5hq6/cd4RjOPddf1G+a931/sJ1/kveLv/29H1J0MXQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCGEEEIIIYQQQgghhBBCCCFE/vl/2ZjU4VXP08MAAAAASUVORK5CYII=",
			Color:       "#111827",
			Tags:        []string{"email"},
		},
		Website: "https://mailgun.com",
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Webhook:  true,
			Platforms: []providers.Platform{providers.PlatformEmail},
			Channels: []providers.Channel{providers.ChannelEmail},
			RateLimit: &providers.RateLimit{
				Limit:    5,
				Interval: "1s",
				Override: true,
			},
			Config: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &modules.JSONSchema{
							Type: "object",
							Properties: []modules.JSONSchemaProperty{
								{
									Name:   "apiKey",
									Schema: &modules.JSONSchema{Type: "string", Title: "Mailgun API Key", Format: "password"},
								},
								{
									Name: "apiRegion",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "API Region",
										Description: "Mailgun API region (US or EU)",
										Enum:        []string{"US", "EU"},
									},
								},
								{
									Name:   "domain",
									Schema: &modules.JSONSchema{Type: "string", Title: "Mailgun Domain", Description: "Verified Mailgun sending domain (e.g. mg.example.com)"},
								},
								{
									Name:   "webhookSigningKey",
									Schema: &modules.JSONSchema{Type: "string", Title: "Webhook Signing Key", Format: "password", Description: "Optional Mailgun webhook signing key used to verify webhook signatures"},
								},
								{
									Name:   "webhookUrl",
									Schema: &modules.JSONSchema{Type: "string", Title: "Webhook URL", Description: "Mailgun webhook callback URL (auto-configured)"},
									Hidden: true,
								},
							},
							Required: []string{"apiKey", "domain"},
						},
					},
				},
			},
		},
	}

	if err := pdk.OutputJSON(manifest); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if req.Channel != providers.ChannelEmail {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return ExitPermanent
	}

	email, err := req.GetEmailPayload()
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if email.To == "" {
		pdk.SetError(fmt.Errorf("missing required 'to' address"))
		return ExitPermanent
	}
	if email.From.Address == "" {
		pdk.SetError(fmt.Errorf("missing required 'from' address"))
		return ExitPermanent
	}
	if email.Subject == "" {
		pdk.SetError(fmt.Errorf("missing required 'subject'"))
		return ExitPermanent
	}

	sendReq, err := ComposeSendEmailRequest(email, req.Config.Domain)
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	apiBase, err := resolveMailgunAPIBase(req.Config.APIRegion)
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	endpoint := fmt.Sprintf("%s/v3/%s/messages", apiBase, sendReq.Domain)
	status, respBody, err := callMailgun(context.Background(), req.Config.APIKey, http.MethodPost, endpoint, sendReq.Form)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to call Mailgun API: %w", err))
		return ExitTransient
	}

	if status < 200 || status >= 300 {
		pdk.SetError(fmt.Errorf("mailgun API error (status=%d): %s", status, string(respBody)))
		return classifyHTTPStatus(status)
	}

	var sendResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		pdk.SetError(fmt.Errorf("failed to decode Mailgun send response: %w", err))
		return ExitTransient
	}

	if err := pdk.OutputJSON(providers.SendResponse{ID: sendResp.ID, Status: "sent"}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:export webhook
func WebhookHandler() int32 {
	var req providers.WebhookRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	if config.WebhookSigningKey != "" {
		if err := verifyMailgunWebhookSignature(config.WebhookSigningKey, req.Body); err != nil {
			pdk.SetError(fmt.Errorf("failed to verify mailgun webhook signature: %w", err))
			return ExitPermanent
		}
	}

	event, ok, err := parseMailgunWebhookEvent(req.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse mailgun webhook body: %w", err))
		return ExitPermanent
	}

	if !ok {
		if err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}}); err != nil {
			pdk.SetError(err)
			return ExitTransient
		}
		return ExitSuccess
	}

	if err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{event}}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:export validate
func Validate() int32 {
	var req providers.ValidateRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	errs := validateConfig(config)

	if len(errs) > 0 {
		if err := pdk.OutputJSON(providers.ValidateResponse{
			Valid:   false,
			Errors:  errs,
			Message: "invalid provider configuration",
		}); err != nil {
			pdk.SetError(err)
			return ExitPermanent
		}
		return ExitSuccess
	}

	if err := pdk.OutputJSON(providers.ValidateResponse{Valid: true}); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	return ExitSuccess
}
