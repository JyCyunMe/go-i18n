package i18n

import "testing"

func TestInitI18nWithOptions(t *testing.T) {
	err := InitI18nWithOptions(nil, NewOptionWithData(DefaultUseSystemLanguage, true))
	if err != nil {
		t.Fatal(err)
	}
}
