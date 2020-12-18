package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/spf13/cobra"
	"github.com/xuperchain/xuperchain/core/utxo"
)

type MiddleCommand struct {
	cli      *Cli
	cmd      *cobra.Command
	action   string
	contract string
	method   string
	args     string
	uuid     string
	amount   string
	name     string
	index    string
}

func NewMiddleCommand(cli *Cli) *cobra.Command {
	c := new(MiddleCommand)
	c.cli = cli
	c.cmd = &cobra.Command{
		Use:   "middle",
		Short: "Operate a command with middle, put|get|del|swap|vote|invoke",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c.action = args[0]
			switch c.action {
			case "put", "swap":
				if len(args) != 3 {
					return errors.New("missing parameter")
				}
				c.name = args[1]
				c.index = args[2]
			case "del":
				if len(args) != 2 {
					return errors.New("missing parameter")
				}
				c.name = args[1]
			case "vote":
				if len(args) != 3 {
					return errors.New("missing parameter")
				}
				c.uuid = args[1]
				c.amount = args[2]
			case "invoke":
				if len(args) != 4 {
					return errors.New("missing parameter")
				}
				c.contract = args[1]
				c.method = args[2]
				c.args = args[3]
			}
			return c.middle(context.TODO())
		},
	}
	c.addFlags()
	return c.cmd
}

func init() {
	AddCommand(NewMiddleCommand)
}

func (c *MiddleCommand) addFlags() {
	c.cmd.Flags().StringVar(&c.contract, "contract", "", "contract name")
	c.cmd.Flags().StringVar(&c.method, "method", "", "method name")
	c.cmd.Flags().StringVarP(&c.args, "args", "a", "{}", "contract method args")
	c.cmd.Flags().StringVar(&c.uuid, "uuid", "", "vote id")
	c.cmd.Flags().StringVar(&c.amount, "amount", "0", "vote amount, will be transfer to contract")
	c.cmd.Flags().StringVar(&c.name, "name", "", "middle contract name")
	c.cmd.Flags().StringVar(&c.index, "index", "", "middle contract sort")
}

func (c *MiddleCommand) example() string {
	return `
./xchain-cli middle get
./xchain-cli middle put middle_contract 0
./xchain-cli middle swap middle_contract 0
./xchain-cli middle del middle_contract
./xchain-cli middle vote uuid 999
./xchain-cli middle invoke contract method '{"key":"value"}'
`
}

func (c *MiddleCommand) middle(ctx context.Context) error {
	if c.amount == "" {
		c.amount = "0"
	}
	account, err := readAddress(c.cli.RootOptions.Keys)
	if err != nil {
		return err
	}
	ct := &CommTrans{
		ModuleName: "middle",
		//ContractName: c.contract,
		//MethodName:   c.method,
		From:         account,
		To:           c.contract,
		Amount:       c.amount,
		Args:         make(map[string][]byte),
		ChainName:    c.cli.RootOptions.Name,
		Keys:         c.cli.RootOptions.Keys,
		XchainClient: c.cli.XchainClient(),
		CryptoType:   c.cli.RootOptions.CryptoType,
		Version:      utxo.TxVersion,
	}

	args := make(map[string]interface{})
	err = json.Unmarshal([]byte(c.args), &args)
	if err != nil {
		return err
	}
	ct.Args, err = convertToXuper3Args(args)
	if err != nil {
		return err
	}

	//传递给中间层使用
	ct.Args["middle_action"] = []byte(c.action)
	ct.Args["middle_contract"] = []byte(c.contract)
	ct.Args["middle_method"] = []byte(c.method)
	ct.Args["middle_uuid"] = []byte(c.uuid)
	ct.Args["middle_amount"] = []byte(c.amount)
	ct.Args["middle_name"] = []byte(c.name)
	ct.Args["middle_index"] = []byte(c.index)
	ct.Args["middle_module"] = []byte("wasm")

	//fmt.Printf("%+v\n", c)
	//for k, v := range ct.Args {
	//	fmt.Println(k, string(v))
	//}

	//查询不需要使到post函数，所以只需要预执行获取数据即可
	if c.action == "get" {
		_, _, err := ct.GenPreExeRes(ctx)
		return err
	}

	//todo 转账操作
	if c.action == "vote" {
		txid, err := c.cli.Transfer(ctx, &TransferOptions{
			BlockchainName: c.cli.RootOptions.Name,
			KeyPath:        c.cli.RootOptions.Keys,
			CryptoType:     c.cli.RootOptions.CryptoType,
			Version:        utxo.TxVersion,
			To:             "$", //转给销毁地址
			//To:           account, //转给自己 //todo v2版本需要改成根据投票时的txid进行金额解冻
			//FrozenHeight: -1,      //永久冻结
			Amount: c.amount,
			Fee:    "0",
			Desc:   []byte("transfer from middle"),
		})
		if err != nil {
			return err
		}
		ct.Args["middle_txid"] = []byte(txid)
	}

	return ct.Transfer(ctx)
}
