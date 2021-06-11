package i18n

import (
	"bufio"
	"fmt"
	"log"
	"os"
	path "path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/BurntSushi/toml"
	goI18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Lang 语言
type Lang struct {
	// Name 语言名称
	Name string
	// Tag 语言标记
	Tag language.Tag
	// fileName 语言包文件名
	fileName string
}

// Data i18n数据
type Data struct {
	// Data 变量map
	Data map[string]interface{}
	// PluralCount 复数
	PluralCount int
}

// CallbackData 回调数据
type CallbackData struct {
	Callback func(params ...interface{})

	CallbackId uint32
	NotOrigin  bool
}

// I18nConfig i18n配置
type I18nConfig struct {
	// Id
	Id string
	// Format 可变格式
	Format string
	// Data i18n数据
	Data *Data
	// CallbackData 回调数据
	//CallbackData    CallbackData
}

var (
	callbackDataMap       map[uint32]*CallbackData
	currentCallbackDataId uint32
)

var (
	Localizer *goI18n.Localizer
	Languages []*Lang

	Language    *Lang
	DefaultLang *Lang

	LogInfoFunc  func(format string, v ...interface{})
	LogErrorFunc func(format string, v ...interface{})

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
	callbackDataMap = make(map[uint32]*CallbackData)
}

// SetDefaultLang 设置默认语言
func SetDefaultLang(lang Lang) Lang {
	DefaultLang = &lang
	return lang
}

// InitI18n 初始化 (语言, Info日志方法, Error日志方法)
func InitI18n(lang *Lang, logInfoFunc func(format string, v ...interface{}), logErrorFunc func(format string, v ...interface{})) {
	InitI18nWithConfig(lang, logInfoFunc, logErrorFunc, "", nil, "")
}

// InitI18nWithConfig 初始化 (语言, Info日志方法, Error日志方法, 语言包格式名, 语言包解码方法, 语言包路径模式)
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

// GTF GetI18nFormatted别名 获取指定Id的本地化文本 (i18n配置)
func GTF(i18nConfig *I18nConfig) (formatted string) {
	return GetI18nFormatted(i18nConfig)
}

// GetI18nFormatted 获取指定Id的本地化文本 (i18n配置)
func GetI18nFormatted(i18nConfig *I18nConfig) (formatted string) {
	formatted = TData(i18nConfig.Id, i18nConfig.Data)
	if len(i18nConfig.Format) > 0 {
		if strings.Contains(i18nConfig.Format, "%s") {
			formatted = fmt.Sprintf(i18nConfig.Format, formatted)
		} else {
			formatted += i18nConfig.Format
		}
	}
	return
}

// T 获取指定Id的本地化文本 (默认文本)
func T(id string) (localize string) {
	return TC("", id)
}

// TC 获取指定Id的本地化文本，未找到则使用默认文本 (默认文本, id)
func TC(defaultLocalized string, id string) (localize string) {
	return Localize(defaultLocalized, id, nil, 0)
}

// TData 获取指定Id的本地化文本，使用i18n数据 (id, i18n数据)
func TData(id string, data *Data) (localize string) {
	return TCData("", id, data)
}

// TCData 获取指定Id的本地化文本，使用i18n数据，未找到则使用默认文本 (默认文本, id, i18n数据)
func TCData(defaultLocalized string, id string, data *Data) (localize string) {
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

// TCallback 添加切换语言自动回调 (id, 回调(本地化文本))
func TCallback(id string, callback func(localized string)) {
	TDataCallback(id, nil, callback)
}

// TCCallback 添加切换语言自动回调，未找到则使用默认文本 (默认值, id, 回调(本地化文本))
func TCCallback(defaultLocalized string, id string, callback func(localized string)) {
	TCDataCallback(defaultLocalized, id, nil, callback)
}

// TDataCallback 添加切换语言自动回调，使用i18n数据 (id, i18n数据, 回调(本地化文本))
func TDataCallback(id string, data *Data, callback func(localized string)) {
	TCDataCallback("", id, data, callback)
}

// TCDataCallback 添加切换语言自动回调，使用i18n数据，未找到则使用默认文本 (默认值, id, i18n数据, 回调(本地化文本))
func TCDataCallback(defaultLocalized string, id string, data *Data, callback func(localized string)) {
	var localizeData map[string]interface{}
	var pluralCount int
	if data != nil {
		localizeData = data.Data
		pluralCount = data.PluralCount
	} else {
		pluralCount = 0
	}
	callback(Localize(defaultLocalized, id, localizeData, pluralCount))

	AddSwitchCallback(&CallbackData{Callback: func(params ...interface{}) {
		TCDataCallback(defaultLocalized, id, data, callback)
	}, NotOrigin: true})
}

// Localize *获取本地化文本，使用变量map和复数，未找到则使用默认文本 (默认值, id, 变量map, 复数)
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

// SwitchLanguage 切换语言 (语言)
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
	for _, callbackData := range callbackDataMap {
		if callbackData != nil && callbackData.Callback != nil {
			callbackData.Callback()
		}
	}
}

// AddSwitchCallbackDo 添加切换语言自动回调，并立即执行回调 (回调数据)
func AddSwitchCallbackDo(data *CallbackData) {
	go AddSwitchCallback(data)
	data.Callback()
}

// AddSwitchCallback 添加切换语言自动回调 (回调数据)
func AddSwitchCallback(data *CallbackData) {
	if data, exist := callbackDataMap[data.CallbackId]; exist {
		if !data.NotOrigin {
			LogErrorFunc("[i18n] cannot add duplicated callback")
		}
		return
	}
	getNewCallbackDataId()
	callbackDataMap[currentCallbackDataId] = data
}

func getNewCallbackDataId() uint32 {
	return atomic.AddUint32(&currentCallbackDataId, 1)
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
