package i18n

import "golang.org/x/text/language"

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

	// DefaultUseSystemLanguage 未获取到语言配置时是否使用系统语言
	DefaultUseSystemLanguage
)

var (
	SimplifiedChinese  = Lang{Name: "zh-Hans", Tag: language.SimplifiedChinese}
	TraditionalChinese = Lang{Name: "zh-Hant", Tag: language.TraditionalChinese}
	English            = Lang{Name: "en", Tag: language.English}
)
