package assistant

import (
	"errors"
)

func cacheStore(key string, value interface{}) error {
	v, err := json.Marshal(value)
	if err != nil {
		return err
	}
	cachedb.SetDefault(key, string(v))
	return nil
}

func cacheGet(key string, value interface{}) error {
	v, ok := cachedb.Get(key)
	if !ok {
		return errors.New("item not exist")
	}
	return json.Unmarshal([]byte(v.(string)), value)
}
