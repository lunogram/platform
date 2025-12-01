package importer

import (
	"encoding/json"
	"strings"

	"github.com/lunogram/platform/services/nexus/internal/store"
)

var UserFieldMap = map[string]func(*store.UpsertUserParams, string){
	"external_id": func(u *store.UpsertUserParams, v string) { u.ExternalID = &v },
	"email":       func(u *store.UpsertUserParams, v string) { u.Email = &v },
	"phone":       func(u *store.UpsertUserParams, v string) { u.Phone = &v },
	"timezone":    func(u *store.UpsertUserParams, v string) { u.Timezone = &v },
	"locale":      func(u *store.UpsertUserParams, v string) { u.Locale = &v },
}

func NewUsers(headers []string) *UserMapper {
	setters := make([]func(*store.UpsertUserParams, string), len(headers))
	data := make([]string, len(headers))

	for index, header := range headers {
		key := strings.ToLower(strings.TrimSpace(header))

		fn, ok := UserFieldMap[key]
		if ok {
			setters[index] = fn
			continue
		}

		data[index] = header
	}

	return &UserMapper{
		Setters: setters,
		Data:    data,
		Headers: headers,
	}
}

type UserMapper struct {
	Setters []func(*store.UpsertUserParams, string)
	Data    []string
	Headers []string
}

func (users *UserMapper) MapRecord(record []string) (store.UpsertUserParams, error) {
	user := store.UpsertUserParams{}
	data := make(map[string]string)

	for index, value := range record {
		value = strings.TrimSpace(value)

		if index < len(users.Setters) && users.Setters[index] != nil {
			users.Setters[index](&user, value)
			continue
		}

		data[users.Data[index]] = value
	}

	if len(data) > 0 {
		b, err := json.Marshal(data)
		if err != nil {
			return user, err
		}
		raw := json.RawMessage(b)
		user.Data = &raw
	}

	return user, nil
}
