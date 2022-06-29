package i18n

import "golang.org/x/text/language"

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
