package importer

import (
	"errors"
	"strings"

	"github.com/lunogram/platform/internal/store/users"
)

var ErrMissingExternalID = errors.New("external_id column is required")

var UserFieldMap = map[string]func(*users.UpsertUserParams, string){
	"external_id": func(u *users.UpsertUserParams, v string) { u.ExternalID = &v },
	"email":       func(u *users.UpsertUserParams, v string) { u.Email = &v },
	"phone":       func(u *users.UpsertUserParams, v string) { u.Phone = &v },
	"timezone":    func(u *users.UpsertUserParams, v string) { u.Timezone = &v },
	"locale":      func(u *users.UpsertUserParams, v string) { u.Locale = &v },
}

func NewUsers(headers []string) (*UserMapper, error) {
	setters := make([]func(*users.UpsertUserParams, string), len(headers))
	data := make([]string, len(headers))
	hasExternalID := false

	for index, header := range headers {
		key := strings.ToLower(strings.TrimSpace(header))

		fn, ok := UserFieldMap[key]
		if ok {
			setters[index] = fn
			if key == "external_id" {
				hasExternalID = true
			}
			continue
		}

		data[index] = header
	}

	if !hasExternalID {
		return nil, ErrMissingExternalID
	}

	return &UserMapper{
		Setters: setters,
		Data:    data,
		Headers: headers,
	}, nil
}

type UserMapper struct {
	Setters []func(*users.UpsertUserParams, string)
	Data    []string
	Headers []string
}

func (m *UserMapper) MapRecord(record []string) (users.UpsertUserParams, error) {
	user := users.UpsertUserParams{}
	user.Data = make(map[string]any)

	for index, value := range record {
		value = strings.TrimSpace(value)

		if index < len(m.Setters) && m.Setters[index] != nil {
			m.Setters[index](&user, value)
			continue
		}

		user.Data[m.Data[index]] = value
	}

	return user, nil
}
