package importer

import (
	"errors"
	"strings"

	"github.com/lunogram/platform/services/nexus/internal/store"
)

var ErrMissingExternalID = errors.New("external_id column is required")

var UserFieldMap = map[string]func(*store.UpsertUserParams, string){
	"external_id": func(u *store.UpsertUserParams, v string) { u.ExternalID = &v },
	"email":       func(u *store.UpsertUserParams, v string) { u.Email = &v },
	"phone":       func(u *store.UpsertUserParams, v string) { u.Phone = &v },
	"timezone":    func(u *store.UpsertUserParams, v string) { u.Timezone = &v },
	"locale":      func(u *store.UpsertUserParams, v string) { u.Locale = &v },
}

func NewUsers(headers []string) (*UserMapper, error) {
	setters := make([]func(*store.UpsertUserParams, string), len(headers))
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
	Setters []func(*store.UpsertUserParams, string)
	Data    []string
	Headers []string
}

func (users *UserMapper) MapRecord(record []string) (store.UpsertUserParams, error) {
	user := store.UpsertUserParams{}
	user.Data = make(map[string]any)

	for index, value := range record {
		value = strings.TrimSpace(value)

		if index < len(users.Setters) && users.Setters[index] != nil {
			users.Setters[index](&user, value)
			continue
		}

		user.Data[users.Data[index]] = value
	}

	return user, nil
}
