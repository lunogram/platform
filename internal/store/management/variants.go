package management

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/render"
)

// VariantSelectorType distinguishes the two ways a send decides which template
// variant it wants. The mode is explicit rather than inferred from whether the
// value happens to contain Liquid syntax: a static key can then be checked
// against the campaign's declared variants when it is saved, and the console
// knows which editor to offer without inspecting the string.
type VariantSelectorType string

const (
	// VariantSelectorStatic pins one variant for every recipient. An empty
	// key pins the default variant, which is how a single send is forced
	// back to house branding past a campaign that resolves a client brand
	// per recipient - a security notice inside an otherwise white-labelled
	// journey, say. That is distinct from carrying no selector at all, which
	// defers to the campaign.
	VariantSelectorStatic VariantSelectorType = "static"
	// VariantSelectorExpression resolves a variant per recipient from a Liquid
	// expression such as "{{ user.data.tenant }}".
	VariantSelectorExpression VariantSelectorType = "expression"
)

// VariantSelector decides which template variant a send uses. The same type
// appears on a campaign, on a journey campaign step and on a broadcast, so the
// three layers offer identical choices; the more specific layer wins.
type VariantSelector struct {
	Type       VariantSelectorType `json:"type"`
	Key        string              `json:"key,omitempty"`
	Expression string              `json:"expression,omitempty"`
}

// Resolve produces the variant key this selector points at, rendering the
// expression against the supplied context when there is one. An empty result
// means the default variant.
func (selector VariantSelector) Resolve(data map[string]any) (string, error) {
	switch selector.Type {
	case VariantSelectorStatic:
		return strings.TrimSpace(selector.Key), nil
	case VariantSelectorExpression:
		resolved, err := render.RenderString(selector.Expression, data)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(resolved), nil
	default:
		return "", fmt.Errorf("unknown variant selector type %q", selector.Type)
	}
}

// Validate reports whether the selector is usable against a campaign's declared
// variants. A static key is checked here so a stale or mistyped one is refused
// when it is saved rather than silently falling back to house branding on every
// send; an expression can only be judged at send time.
func (selector VariantSelector) Validate(variants CampaignVariants) error {
	switch selector.Type {
	case VariantSelectorStatic:
		// Has reports true for the empty key, so pinning the default variant
		// needs no special case here.
		if key := strings.TrimSpace(selector.Key); !variants.Has(key) {
			return fmt.Errorf("campaign does not declare variant %s", key)
		}
		return nil
	case VariantSelectorExpression:
		if strings.TrimSpace(selector.Expression) == "" {
			return fmt.Errorf("expression variant selector requires an expression")
		}
		return nil
	default:
		return fmt.Errorf("unknown variant selector type %q", selector.Type)
	}
}

// variantKeyPattern constrains a declared variant key. Keys are written into
// template rows and into console dropdown values, so they are kept to a plain
// slug: leading punctuation is refused, which keeps the sentinel values the
// console reserves for "default" and "none" out of the declared set.
var variantKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// CampaignVariant declares one white-labelled edition of a campaign. Key is
// what a send resolves against to pick a set of templates; the empty key is the
// default variant every campaign starts with and is never declared here.
type CampaignVariant struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
}

// CampaignVariants holds a campaign's declared variants together with the rule
// that picks between them when a send does not pick one itself. They are one
// concept - a selector is meaningless without the set it resolves into - so
// they are stored and edited as one object.
type CampaignVariants struct {
	Selector *VariantSelector  `json:"selector,omitempty"`
	Options  []CampaignVariant `json:"options,omitempty"`
}

// Has reports whether key names a declared variant. The default variant is
// always available and is not part of the declared set.
func (variants CampaignVariants) Has(key string) bool {
	if key == "" {
		return true
	}
	for _, option := range variants.Options {
		if option.Key == key {
			return true
		}
	}
	return false
}

func (selector VariantSelector) OAPI() oapi.VariantSelector {
	result := oapi.VariantSelector{Type: oapi.VariantSelectorType(selector.Type)}
	if selector.Key != "" {
		result.Key = &selector.Key
	}
	if selector.Expression != "" {
		result.Expression = &selector.Expression
	}
	return result
}

func VariantSelectorFromOAPI(selector oapi.VariantSelector) VariantSelector {
	result := VariantSelector{Type: VariantSelectorType(selector.Type)}
	if selector.Key != nil {
		result.Key = strings.TrimSpace(*selector.Key)
	}
	if selector.Expression != nil {
		result.Expression = strings.TrimSpace(*selector.Expression)
	}
	return result
}

func (variants CampaignVariants) OAPI() oapi.CampaignVariants {
	options := make([]oapi.CampaignVariant, len(variants.Options))
	for i, option := range variants.Options {
		options[i] = oapi.CampaignVariant{Key: option.Key}
		if option.Label != "" {
			options[i].Label = &variants.Options[i].Label
		}
	}

	result := oapi.CampaignVariants{Options: &options}
	if variants.Selector != nil {
		selector := variants.Selector.OAPI()
		result.Selector = &selector
	}
	return result
}

// CampaignVariantsFromOAPI converts a request body into the stored shape,
// rejecting duplicate and empty keys. The empty key is the default variant: it
// always exists and is never declared, so accepting it here would create a
// duplicate of something that cannot be removed.
func CampaignVariantsFromOAPI(variants oapi.CampaignVariants) (CampaignVariants, error) {
	var result CampaignVariants

	if variants.Options != nil {
		result.Options = make([]CampaignVariant, 0, len(*variants.Options))
		seen := make(map[string]bool, len(*variants.Options))

		for _, option := range *variants.Options {
			key := strings.TrimSpace(option.Key)
			if key == "" {
				return result, fmt.Errorf("variant key cannot be empty")
			}
			if !variantKeyPattern.MatchString(key) {
				return result, fmt.Errorf("variant key %s must start with a lowercase letter or digit and contain only lowercase letters, digits, dashes and underscores", key)
			}
			if seen[key] {
				return result, fmt.Errorf("duplicate variant key %s", key)
			}
			seen[key] = true

			converted := CampaignVariant{Key: key}
			if option.Label != nil {
				converted.Label = strings.TrimSpace(*option.Label)
			}
			result.Options = append(result.Options, converted)
		}
	}

	if variants.Selector != nil {
		selector := VariantSelectorFromOAPI(*variants.Selector)
		if err := selector.Validate(result); err != nil {
			return result, err
		}
		result.Selector = &selector
	}

	return result, nil
}
