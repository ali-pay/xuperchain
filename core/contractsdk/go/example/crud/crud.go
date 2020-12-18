package main

import (
	"bytes"

	"github.com/xuperchain/xuperchain/core/contractsdk/go/code"
	"github.com/xuperchain/xuperchain/core/contractsdk/go/driver"
)

const (
	mapKey       = "key"
	mapValue     = "value"
	missingKey   = "missing key"
	missingValue = "missing value"
	notFound     = "Key not found"
	success      = "success"
)

type crud struct{}

func (c *crud) Initialize(ctx code.Context) code.Response {
	return code.OK([]byte("initialize success"))
}

func (c *crud) Version(ctx code.Context) code.Response {
	return code.OK([]byte("1.0"))
}

func (c *crud) Put(ctx code.Context) code.Response {
	key, ok := ctx.Args()[mapKey]
	if !ok {
		return code.Errors(missingKey)
	}
	value, ok := ctx.Args()[mapValue]
	if !ok {
		return code.Errors(missingValue)
	}
	err := ctx.PutObject(key, value)
	if err != nil {
		return code.Error(err)
	}
	return code.OK([]byte(success))
}

func (c *crud) Get(ctx code.Context) code.Response {
	key, ok := ctx.Args()[mapKey]
	if !ok {
		return code.Errors(missingKey)
	}
	value, err := ctx.GetObject(key)
	if err != nil {
		return code.Error(err)
	}
	return code.OK(value)
}

func (c *crud) GetByPrefix(ctx code.Context) code.Response {
	key, ok := ctx.Args()[mapKey]
	if !ok {
		return code.Errors(missingKey)
	}
	it := ctx.NewIterator(code.PrefixRange(key))
	if it.Error() != nil {
		return code.Error(it.Error())
	}
	defer it.Close()
	var values [][]byte
	for it.Next() {
		values = append(values, it.Value())
	}
	if len(values) == 0 {
		return code.Errors(notFound)
	}
	data := bytes.Join(values, []byte(","))
	values = [][]byte{[]byte("["), data, []byte("]")}
	return code.OK(bytes.Join(values, nil))
}

func (c *crud) GetMapByPrefix(ctx code.Context) code.Response {
	key, ok := ctx.Args()[mapKey]
	if !ok {
		return code.Errors(missingKey)
	}
	it := ctx.NewIterator(code.PrefixRange(key))
	if it.Error() != nil {
		return code.Error(it.Error())
	}
	defer it.Close()
	values := make(map[string][]byte)
	for it.Next() {
		values[string(it.Key())] = it.Value()
	}
	if len(values) == 0 {
		return code.Errors(notFound)
	}
	return code.JSON(values)
}

func main() {
	driver.Serve(new(crud))
}
