package i18n

import (
	"bufio"
	"fmt"
	"log"
	"os"
	path "path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	goI18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type Lang struct {
	Name     string
	Tag      language.Tag
	fileName string
}

type Data struct {
	Data        map[string]interface{}
	PluralCount int
}

var (
	Localizer *goI18n.Localizer
	Languages []*Lang

	Language    *Lang
	DefaultLang *Lang

	LogInfoFunc     func(format string, v ...interface{})
	LogErrorFunc    func(format string, v ...interface{})
	SwitchCallbacks []func()

	bundle          *goI18n.Bundle
	i18nLabelRegexp *regexp.Regexp
)

var (
	SimplifiedChinese = Lang{Name: "zh-Hans", Tag: language.SimplifiedChinese}
	English           = Lang{Name: "en", Tag: language.English}
)

func init() {
	var err error
	i18nLabelRegexp, err = regexp.Compile(`# \[i18n\] <(.*)> <(.*)>`)
	if err != nil {
		LogErrorFunc("[i18n] init i18nLabelRegexp Compile error: %v", err)
		panic(fmt.Errorf("[i18n] init i18nLabelRegexp Compile error: %v", err))
	}
}

func SetDefaultLang(lang Lang) Lang {
	DefaultLang = &lang
	return lang
}

func InitI18n(lang *Lang, logInfoFunc func(format string, v ...interface{}), logErrorFunc func(format string, v ...interface{})) {
	InitI18nWithConfig(lang, logInfoFunc, logErrorFunc, "", nil, "")
}

func InitI18nWithConfig(lang *Lang, logInfoFunc func(format string, v ...interface{}), logErrorFunc func(format string, v ...interface{}),
	format string, unmarshalFunc goI18n.UnmarshalFunc, langFilesPattern string) {
	if logInfoFunc == nil {
		LogInfoFunc = log.Printf
	} else {
		LogInfoFunc = logInfoFunc
	}
	if logErrorFunc == nil {
		LogErrorFunc = log.Fatalf
	} else {
		LogErrorFunc = logErrorFunc
	}
	if len(format) == 0 {
		format = "lang"
	}
	if unmarshalFunc == nil {
		unmarshalFunc = toml.Unmarshal
	}
	if len(langFilesPattern) == 0 {
		langFilesPattern = "./lang/*.lang"
	}
	if lang == nil {
		lang = &SimplifiedChinese
		LogInfoFunc("[i18n] Not special language, default using %s (%s)", DefaultLang.Name, DefaultLang.Tag.String())
	}
	SetDefaultLang(*lang)
	initI18n(lang, format, unmarshalFunc, langFilesPattern)
}

func initI18n(lang *Lang, format string, unmarshalFunc goI18n.UnmarshalFunc, langFilesPattern string) {
	LogInfoFunc("[i18n] InitI18n started")
	bundle = goI18n.NewBundle(lang.Tag)
	bundle.RegisterUnmarshalFunc(format, unmarshalFunc)
	languages, err := path.Glob(langFilesPattern)
	if err != nil {
		LogErrorFunc("[i18n] InitI18n Glob error: %v", err)
	}
	for _, langFile := range languages {
		labels := readLangLabel(langFile)
		if len(labels) < 2 {
			LogErrorFunc("[i18n] InitI18n readLangLabel file is invalid: %s", langFile)
			continue
		}
		lang := &Lang{
			Name:     labels[1],
			Tag:      parseTags(labels[0])[0],
			fileName: langFile,
		}
		langName := fmt.Sprintf("%s (%s)", lang.Name, lang.Tag.String())
		LogInfoFunc("[i18n] Found language: %s", langName)
		if Language == nil && DefaultLang != nil && DefaultLang.Tag == lang.Tag {
			Language = lang
			_, err = bundle.LoadMessageFile(langFile)
			if err != nil {
				LogErrorFunc("[i18n] initI18n load language file \"%s\" error: %v", langName, err)
				return
			}
			Localizer = goI18n.NewLocalizer(bundle, lang.Tag.String())
			languages := []*Lang{lang}
			Languages = append(languages, Languages...)
			LogInfoFunc("[i18n] Loaded default language %s", langName)
		} else {
			Languages = append(Languages, lang)
		}
	}
	LogInfoFunc("[i18n] Loaded %d language(s)", len(Languages))
	if Language == nil {
		LogErrorFunc("[i18n] Cannot load any language")
		panic("[i18n] Cannot load any language")
	}
	LogInfoFunc("[i18n] InitI18n finished")
}

func T(id string) (localize string) {
	return TC("", id)
}

func TC(defaultLocalized string, id string) (localize string) {
	return Localize(defaultLocalized, id, nil, 0)
}

func TData(defaultLocalized string, id string, data *Data) (localize string) {
	var localizeData map[string]interface{}
	var pluralCount int
	if data != nil {
		localizeData = data.Data
		pluralCount = data.PluralCount
	} else {
		pluralCount = 0
	}
	return Localize(defaultLocalized, id, localizeData, pluralCount)
}

func Localize(defaultLocalized string, id string, data map[string]interface{}, pluralCount int) (localized string) {
	localized, err := Localizer.Localize(&goI18n.LocalizeConfig{
		DefaultMessage: &goI18n.Message{
			ID: id,
		},
		TemplateData: data,
		PluralCount:  pluralCount,
	})
	if err != nil {
		LogErrorFunc("[i18n] i18n error: %v", err)
		return defaultLocalized
	}
	return localized
}

func SwitchLanguage(lang *Lang) {
	langName := fmt.Sprintf("%s (%s)", lang.Name, lang.Tag.String())
	_, err := bundle.LoadMessageFile(lang.fileName)
	if err != nil {
		LogErrorFunc("[i18n] SwitchLanguage load language file \"%s\" error: %v", langName, err)
		return
	}
	Localizer = goI18n.NewLocalizer(bundle, lang.Tag.String())
	Language = lang
	LogInfoFunc("[i18n] switched to new language: %s", langName)
	for _, callback := range SwitchCallbacks {
		if callback != nil {
			callback()
		}
	}
}

func AddSwitchCallback(callback func()) {
	SwitchCallbacks = append(SwitchCallbacks, callback)
}

func readLangLabel(fileName string) []string {
	f, err := os.Open(fileName)
	if err != nil {
		LogErrorFunc("[i18n] readLangLabel error: %v", err)
		panic(err)
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	topLine, _ := rd.ReadString('\n')
	if i18nLabelRegexp.MatchString(topLine) {
		return i18nLabelRegexp.FindStringSubmatch(topLine)[1:3]
	}
	return []string{}
}

func parseTags(lang string) (tags []language.Tag) {
	tags = []language.Tag{}
	t, _, err := language.ParseAcceptLanguage(lang)
	if err != nil {
		return
	}
	return t
}
