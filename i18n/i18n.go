package i18n

import (
	"bufio"
	"bytes"
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

	// FileName 语言包文件名
	FileName string

	// Data 数据
	Data *[]byte
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
	// 回调方法
	Callback func(params ...interface{})

	// 回调Id
	CallbackId uint32
	// 非原始回调标识 (内部用)
	notOrigin bool
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

// OptionType 选项类型
type OptionType int8

// Option 选项
type Option struct {

	// OptionType 选项类型
	OptionType OptionType

	// Data 选项数据
	Data *interface{}

	// Callback 选项回调
	Callback func(v interface{}) interface{}
}

const (

	// OptionType 选项类型枚举

	// LogInfoFunc Info日志方法
	LogInfoFunc OptionType = iota + 1

	// LogErrorFunc Error日志方法
	LogErrorFunc

	// UnmarshalFunc 语言包解码方法
	UnmarshalFunc

	// PackageSuffix 语言包格式名
	PackageSuffix

	// PackagePath 语言包路径
	PackagePath

	// PackagePattern 语言包路径正则表达式
	PackagePattern

	// PackageListFunc 语言包列表方法
	PackageListFunc
)

var (
	callbackDataMap       map[uint32]*CallbackData
	currentCallbackDataId uint32
)

var (
	Localizer *goI18n.Localizer
	// 语言集
	Languages []*Lang

	// 当前语言
	Language *Lang
	// 默认语言
	DefaultLang *Lang

	logInfoFunc     func(format string, v ...interface{})
	logErrorFunc    func(format string, v ...interface{})
	unmarshalFunc   func(p []byte, v interface{}) error
	packageListFunc func(options ...Option) ([]*Lang, error)

	bundle          *goI18n.Bundle
	i18nLabelRegexp *regexp.Regexp
)

var (
	SimplifiedChinese  = Lang{Name: "zh-Hans", Tag: language.SimplifiedChinese}
	TraditionalChinese = Lang{Name: "zh-Hans", Tag: language.TraditionalChinese}
	English            = Lang{Name: "en", Tag: language.English}
)

func init() {
	var err error
	i18nLabelRegexp, err = regexp.Compile(`# \[i18n\] <(.*)> <(.*)>`)
	if err != nil {
		logErrorFunc("[i18n] init i18nLabelRegexp Compile error: %v", err)
		panic(fmt.Errorf("[i18n] init i18nLabelRegexp Compile error: %v", err))
	}
	callbackDataMap = make(map[uint32]*CallbackData)
}

func (l Lang) FullName() string {
	if len(l.Name) > 0 {
		return fmt.Sprintf("%s (%s)", l.Name, l.Tag.String())
	} else {
		return l.Tag.String()
	}
}

// SetDefaultLang 设置默认语言
func SetDefaultLang(lang Lang) Lang {
	DefaultLang = &lang
	return lang
}

func NewOptionWithCallback(optionType OptionType, callback func(v interface{}) interface{}) Option {
	return Option{OptionType: optionType, Callback: callback}
}

func NewOptionWithData(optionType OptionType, data interface{}) Option {
	return Option{OptionType: optionType, Data: &data}
}

// InitI18nWithLogFunc 初始化带日志方法 (语言, Info日志方法, Error日志方法)
func InitI18nWithLogFunc(lang *Lang,
	logInfoFunc func(format string, v ...interface{}),
	logErrorFunc func(format string, v ...interface{})) error {
	return InitI18nWithOptions(lang,
		NewOptionWithCallback(LogInfoFunc, func(v interface{}) interface{} {
			args := v.([]interface{})
			if len(args) > 1 {
				logInfoFunc(args[0].(string), args[1:])
			} else {
				logInfoFunc(args[0].(string))
			}
			return nil
		}), NewOptionWithCallback(LogErrorFunc, func(v interface{}) interface{} {
			args := v.([]interface{})
			if len(args) > 1 {
				logErrorFunc(args[0].(string), args[1:])
			} else {
				logErrorFunc(args[0].(string))
			}
			return nil
		}))
}

// InitI18nWithAllFunc 初始化带所有方法 (语言, Info日志方法, Error日志方法, 反序列化方法, 列举语言包方法)
func InitI18nWithAllFunc(lang *Lang,
	logInfoFunc func(format string, v ...interface{}),
	logErrorFunc func(format string, v ...interface{}),
	unmarshalFunc func(p []byte, v interface{}) error,
	packageListFunc func(options ...Option) ([]*Lang, error)) error {
	var lInfoFunc, lErrorFunc, unFunc, pListFunc func(v interface{}) interface{}
	var options []Option
	if logInfoFunc != nil {
		lInfoFunc = func(v interface{}) interface{} {
			args := v.([]interface{})
			if len(args) > 1 {
				logInfoFunc(args[0].(string), args[1:]...)
			} else {
				logInfoFunc(args[0].(string))
			}
			return nil
		}
		options = append(options, NewOptionWithCallback(LogInfoFunc, lInfoFunc))
	}
	if logErrorFunc != nil {
		lErrorFunc = func(v interface{}) interface{} {
			args := v.([]interface{})
			if len(args) > 1 {
				logErrorFunc(args[0].(string), args[1:]...)
			} else {
				logErrorFunc(args[0].(string))
			}
			return nil
		}
		options = append(options, NewOptionWithCallback(LogErrorFunc, lErrorFunc))
	}
	if unmarshalFunc != nil {
		unFunc = func(v interface{}) interface{} {
			args := v.([]interface{})
			return unmarshalFunc(args[0].([]byte), args[1])
		}
		options = append(options, NewOptionWithCallback(UnmarshalFunc, unFunc))
	}
	if packageListFunc != nil {
		pListFunc = func(v interface{}) interface{} {
			langs, err := packageListFunc(options...)
			return []interface{}{langs, err}
		}
		options = append(options, NewOptionWithCallback(PackageListFunc, pListFunc))
	}
	return InitI18nWithOptions(lang, options...)
}

// InitI18nWithOptions 初始化 (语言, Info日志方法, Error日志方法, 语言包格式名, 语言包解码方法, 语言包路径模式)
func InitI18nWithOptions(lang *Lang, options ...Option) error {
	logInfoFunc = log.Printf
	logErrorFunc = log.Fatalf
	unmarshalFunc = toml.Unmarshal
	var pListFunc func(options ...Option) ([]*Lang, error)
	format := "lang"
	langFilesPattern := path.Join("./lang/*." + format)

	existPackagePath := false
	for _, option := range options {
		option := option
		switch option.OptionType {
		case LogInfoFunc:
			logInfoFunc = func(format string, v ...interface{}) {
				args := append([]interface{}{format}, v...)
				option.Callback(args)
			}
			break
		case LogErrorFunc:
			logErrorFunc = func(format string, v ...interface{}) {
				args := append([]interface{}{format}, v...)
				option.Callback(args)
			}
			break
		case UnmarshalFunc:
			unmarshalFunc = func(p []byte, v interface{}) (e error) {
				args := append([]interface{}{p}, v)
				result := option.Callback(args)
				if result != nil {
					e = result.(error)
				}
				return
			}
			break
		case PackageSuffix:
			format = (*option.Data).(string)
			if existPackagePath {
				langFilesPattern = path.Join((*option.Data).(string), "*."+format)
			}
			break
		case PackagePath:
			langFilesPattern = path.Join((*option.Data).(string), "*."+format)
			existPackagePath = true
			break
		case PackagePattern:
			langFilesPattern = (*option.Data).(string)
			break
		case PackageListFunc:
			pListFunc = func(options ...Option) (l []*Lang, e error) {
				args := []interface{}{options}
				result := option.Callback(args).([]interface{})
				if result[0] != nil {
					l = result[0].([]*Lang)
				}
				if result[1] != nil {
					e = result[1].(error)
				}
				return
			}
			break
		default:
			continue
		}
	}
	if pListFunc == nil {
		packageListFunc = func(options ...Option) ([]*Lang, error) {
			return PackageListByPatternFunc(NewOptionWithData(PackagePattern, langFilesPattern))
		}
	} else {
		packageListFunc = func(options ...Option) ([]*Lang, error) {
			return pListFunc(NewOptionWithData(PackagePattern, langFilesPattern))
		}
	}
	if lang == nil {
		lang = &English
		logInfoFunc("[i18n] Not special language, default using %s (%s)", lang.Name, lang.Tag.String())
	}
	SetDefaultLang(*lang)
	logInfoFunc("[i18n] InitI18n started")
	bundle = goI18n.NewBundle(lang.Tag)
	bundle.RegisterUnmarshalFunc(format, unmarshalFunc)
	logInfoFunc("[i18n] Registered unmarshal func for %s", format)
	packageList, err := packageListFunc()
	if err != nil {
		panic(fmt.Sprintf("[i18n] PackageList error: %v", err))
	}
	Languages = packageList
	if len(Languages) == 0 {
		logErrorFunc("[i18n] Cannot load any language")
		panic("[i18n] Cannot load any language")
	}
	for _, l := range Languages {
		//if Language == nil && DefaultLang != nil && DefaultLang.Tag == l.Tag {
		if lang.Tag == l.Tag {
			err := UseLanguage(l)
			if err != nil {
				return err
			}
			break
		}
	}
	logInfoFunc("[i18n] InitI18n finished")
	return nil
}

func PackageListByPatternFunc(options ...Option) ([]*Lang, error) {
	var langFilesPattern string
	for _, option := range options {
		switch option.OptionType {
		case PackagePattern:
			langFilesPattern = (*option.Data).(string)
			break
		default:
			continue
		}
	}
	if len(langFilesPattern) == 0 {
		err := fmt.Errorf("[i18n] PackageList cannot find pattern")
		logErrorFunc(err.Error())
		return nil, err
	}
	packages, err := path.Glob(langFilesPattern)
	if err != nil {
		err = fmt.Errorf("[i18n] PackageList Glob error: %v", err)
		logErrorFunc(err.Error())
		return nil, err
	}
	var languages []*Lang
	for _, langFile := range packages {
		lang := ReadLangFromFileName(langFile)
		if lang == nil {
			continue
		}
		langName := fmt.Sprintf("%s (%s)", lang.Name, lang.Tag.String())
		logInfoFunc("[i18n] Found language: %s", langName)
		languages = append(languages, lang)
	}
	logInfoFunc("[i18n] Found %d language(s)", len(languages))
	return languages, nil
}

func LoadLanguage(lang *Lang) (err error) {
	langFile := lang.FileName
	if lang.Data != nil {
		_, err = bundle.ParseMessageFileBytes(*lang.Data, langFile)
	} else {
		_, err = bundle.LoadMessageFile(langFile)
	}
	if err != nil {
		err = fmt.Errorf("[i18n] Load language file \"%s\" error: %v", lang.FullName(), err)
		logErrorFunc(err.Error())
		return err
	}
	return nil
}

func UseLanguage(lang *Lang) (err error) {
	//if Language == nil && DefaultLang != nil && DefaultLang.Tag == lang.Tag {
	Language = lang
	err = LoadLanguage(lang)
	if err != nil {
		return err
	}
	Localizer = goI18n.NewLocalizer(bundle, lang.Tag.String())
	logInfoFunc("[i18n] Used language %s", lang.FullName())
	//logInfoFunc("[i18n] Used default language %s", langName)
	//}
	return nil
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
	}, notOrigin: true})
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
		logErrorFunc("[i18n] i18n error: %v", err)
		return defaultLocalized
	}
	return localized
}

// SwitchLanguage 切换语言 (语言)
func SwitchLanguage(lang *Lang) (err error) {
	err = LoadLanguage(lang)
	if err != nil {
		return err
	}
	Localizer = goI18n.NewLocalizer(bundle, lang.Tag.String())
	Language = lang
	logInfoFunc("[i18n] switched to new language: %s", lang.FullName())
	for _, callbackData := range callbackDataMap {
		if callbackData != nil && callbackData.Callback != nil {
			callbackData.Callback()
		}
	}
	return nil
}

// AddSwitchCallbackDo 添加切换语言自动回调，并立即执行回调 (回调数据)
func AddSwitchCallbackDo(data *CallbackData) {
	go AddSwitchCallback(data)
	data.Callback()
}

// AddSwitchCallback 添加切换语言自动回调 (回调数据)
func AddSwitchCallback(data *CallbackData) {
	if data, exist := callbackDataMap[data.CallbackId]; exist {
		if !data.notOrigin {
			logErrorFunc("[i18n] cannot add duplicated callback")
		}
		return
	}
	getNewCallbackDataId()
	callbackDataMap[currentCallbackDataId] = data
}

func getNewCallbackDataId() uint32 {
	return atomic.AddUint32(&currentCallbackDataId, 1)
}

func ReadLangLabel(topLine string) []string {
	if i18nLabelRegexp.MatchString(topLine) {
		return i18nLabelRegexp.FindStringSubmatch(topLine)[1:3]
	}
	return []string{}
}

func ReadLangFromFileName(fileName string) *Lang {
	f, err := os.Open(fileName)
	if err != nil {
		logErrorFunc("[i18n] ReadLangFromFileName error: %v", err)
		panic(err)
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	topLine, err := rd.ReadString('\n')
	if err != nil {
		logErrorFunc("[i18n] ReadLangFromFileName error: %v", err)
		return nil
	}

	labels := ReadLangLabel(topLine)
	if len(labels) < 2 {
		logErrorFunc("[i18n] ReadLangFromFileName file is invalid: %s", fileName)
		return nil
	}
	return &Lang{
		Name:     labels[1],
		Tag:      parseTags(labels[0])[0],
		FileName: fileName,
	}
}

// ReadLangFromBytes 从字节数组获取语言包 (字节数组引用, 文件名)
// MessageFile需要从文件名截取语言包格式，已匹配到适配对应格式的反序列化方法
func ReadLangFromBytes(data *[]byte, fileName string) *Lang {
	buf := bytes.NewBuffer(nil)
	for _, b := range *data {
		if b == '\r' || b == '\n' {
			break
		}
		buf.WriteByte(b)
	}
	labels := ReadLangLabel(buf.String())
	buf.Reset()
	if len(labels) < 2 {
		logErrorFunc("[i18n] PackageList ReadLangLabel file from %d bytes is invalid", len(*data))
		return nil
	}
	return &Lang{
		Name:     labels[1],
		Tag:      parseTags(labels[0])[0],
		FileName: fileName,
	}
}

func parseTags(lang string) (tags []language.Tag) {
	tags = []language.Tag{}
	t, _, err := language.ParseAcceptLanguage(lang)
	if err != nil {
		return
	}
	return t
}
