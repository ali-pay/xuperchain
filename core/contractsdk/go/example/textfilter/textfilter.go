package main

import (
	"fmt"
	"strings"

	"github.com/xuperchain/xuperchain/core/contractsdk/go/code"
	"github.com/xuperchain/xuperchain/core/contractsdk/go/driver"
)

var keyPrefix = []byte("textfilter_")

type textfilter struct {
	textlib []string //敏感词词库
}

//初始化词库
func (c *textfilter) Initialize(ctx code.Context) code.Response {
	c.textlib = make([]string, 0)
	return code.OK(nil)
}

func (c *textfilter) Version(ctx code.Context) code.Response {
	return code.OK([]byte("1.0"))
}

//中间件执行函数
func (c *textfilter) Middleware(ctx code.Context) code.Response {
	//遍历请求中的所有参数
	for key, value := range ctx.Args() {
		//result, err := ctx.GetObject(append(keyPrefix, []byte(key)...))
		//if err == nil && result != nil {
		//	return code.Errors(fmt.Sprintf("text_filter is not pass, the param contain: %s", string(result)))
		//}
		//result, err = ctx.GetObject(append(keyPrefix, []byte(value)...))
		//if err == nil && result != nil {
		//	return code.Errors(fmt.Sprintf("text_filter is not pass, the param contain: %s", string(result)))
		//}

		//获取所有敏感词
		if err := c.get(ctx); err != nil {
			//无法获取敏感词时，放行所有请求
			return code.OK(nil)
		}
		//对比请求参数是否匹配敏感词
		for _, v := range c.textlib {
			if strings.Contains(key, v) {
				return code.Errors(fmt.Sprintf("text_filter is not pass, the param contain: %s", v))
			}
			if strings.Contains(string(value), v) {
				return code.Errors(fmt.Sprintf("text_filter is not pass, the param contain: %s", v))
			}
		}
	}
	return code.OK(nil)
}

//添加敏感词
func (c *textfilter) Put(ctx code.Context) code.Response {
	//获取传入的敏感词
	data, ok := ctx.Args()["text"]
	if !ok {
		return code.Errors("missing text")
	}

	//检查是否已存在
	result, err := ctx.GetObject(append(keyPrefix, data...))
	if err == nil && result != nil {
		return code.Errors("the text already exists")
	}

	//存入账本 todo: 空间换时间
	err = ctx.PutObject(append(keyPrefix, data...), data)
	if err != nil {
		return code.Error(err)
	}
	return code.OK(nil)
}

//获取所有敏感词（内部调用）
func (c *textfilter) get(ctx code.Context) error {
	it := ctx.NewIterator(code.PrefixRange(keyPrefix))
	if it.Error() != nil {
		return it.Error()
	}
	defer it.Close()

	for it.Next() {
		c.textlib = append(c.textlib, string(it.Value()))
	}
	return nil
}

//获取所有敏感词（外部调用）
func (c *textfilter) Get(ctx code.Context) code.Response {
	if err := c.get(ctx); err != nil {
		return code.Error(err)
	}
	return code.JSON(c.textlib)
}

func main() {
	driver.Serve(new(textfilter))
}
