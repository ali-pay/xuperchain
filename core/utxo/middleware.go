package utxo

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"

	log "github.com/xuperchain/log15"
	"github.com/xuperchain/xuperchain/core/contract"
	"github.com/xuperchain/xuperchain/core/pb"
	"github.com/xuperchain/xuperchain/core/utxo/txhash"
	"github.com/xuperchain/xuperchain/core/xmodel"
)

//投票人
type Vote struct {
	Auth    []string //签名列表
	Address string   //投票人地址
	Amount  *big.Int //投票金额
	Txid    string   //投票时的交易id //todo v2版本需要改成根据投票时的txid进行金额解冻
}

//投票的实体
type Proposal struct {
	Vote      *Vote             //该动作创建人/投票人
	Action    string            //执行动作
	Module    string            //虚拟机名称
	Contract  string            //合约名称
	Method    string            //合约方法
	Args      map[string][]byte //参数列表
	UUID      string            //该动作的投票id
	Votes     []*Vote           //该动作的投票列表
	VoteCount *big.Int          //获得的总投票金额
	PassCount *big.Int          //投票允许通过的额度
	Name      string            //中间层合约名称
	Index     string            //中间层合约排序顺序
}

//中间层结构
type Middleware struct {
	uv          *UtxoVM              //
	locker      *sync.Mutex          //合约组模块的互斥锁
	xlog        log.Logger           //日志打印
	middlewares []string             //中间件列表：中间层的所有中间件合约名
	votePool    map[string]*Proposal //投票池 key：投票id
	mlocker     *sync.Mutex          //管理模块的互斥锁
	vatlist     map[string]*big.Int  //退款列表 address->amount
}

const middlewareFunc = "middleware" //调用中间件合约的处理函数名称

var (
	keyMiddlewares = []byte("middlewares") //中间件列表落盘时存的key
	middle         *Middleware             //中间层的单例模式
)

//初始化中间层
func NewMiddleware(uv *UtxoVM) *Middleware {
	//判断是否需要初始化，使用单例模式
	if middle != nil {
		return middle
	}

	//初始化参数
	middle = new(Middleware)
	middle.uv = uv
	middle.locker = &sync.Mutex{}
	middle.xlog = log.New("module", "middleware")
	middle.xlog.SetHandler(log.StreamHandler(os.Stderr, log.LogfmtFormat()))
	middle.middlewares = make([]string, 0)
	middle.votePool = make(map[string]*Proposal)
	middle.mlocker = &sync.Mutex{}
	middle.vatlist = make(map[string]*big.Int)

	//注册vat
	uv.RegisterVAT("Middleware", middle, nil)

	//获取账本中已存在的中间件列表
	data, err := middle.uv.ledger.GetBaseDB().Get(keyMiddlewares)
	if err == nil && data != nil {
		if err := json.Unmarshal(data, &middle.middlewares); err != nil {
			panic(err)
		}
	}

	//不存在敏感词中间件时，默认加入
	for _, v := range middle.middlewares {
		//存在时则退出
		if v == "text_filter" {
			return middle
		}
	}
	//不存在时则加入
	middle.middlewares = append(middle.middlewares, "text_filter")
	return middle
}

//修改中间件执行顺序
func (c *Middleware) swap(name, index string) (string, error) {
	//避免空值
	if name == "" {
		return "", fmt.Errorf("the name can not be empty")
	}
	if index == "" {
		index = "0"
	}

	//字符串转成整数
	i, err := strconv.Atoi(index)
	if err != nil {
		return "", err
	}

	//避免索引越界错误
	if i >= len(c.middlewares) || i < 0 {
		return "", fmt.Errorf("the index is invalid, index: %s", index)
	}

	position := -1 //该中间件原位置
	for i, v := range c.middlewares {
		if v == name {
			position = i
			break
		}
	}
	//不存在
	if position == -1 {
		return "", fmt.Errorf("the middleware not exist, name: %s", name)
	}

	//排序
	before := c.middlewares[:i]                                                     //元素之前的值
	c.middlewares = append(c.middlewares[:position], c.middlewares[position+1:]...) //剔除该元素
	after := c.middlewares[i:]                                                      //元素之后的值

	//重新赋值中间件列表
	c.middlewares = []string{}
	c.middlewares = append(c.middlewares, before...)
	c.middlewares = append(c.middlewares, name)
	c.middlewares = append(c.middlewares, after...)

	//落盘中间件列表
	j, _ := json.Marshal(c.middlewares)
	if err := c.uv.ledger.GetBaseDB().Put(keyMiddlewares, j); err != nil {
		return "", err
	}
	return c.get(), nil
}

//新增中间件
func (c *Middleware) put(name, index string) (string, error) {
	//避免空值
	if name == "" {
		return "", fmt.Errorf("the name can not be empty")
	}
	//判断该中间件是否已存在
	for i, v := range c.middlewares {
		if v == name {
			return "", fmt.Errorf("the middleware already exists, name: %s, index: %d", name, i)
		}
	}
	//加入列表
	c.middlewares = append(c.middlewares, name)
	//设置顺序
	return c.swap(name, index)
}

//删除中间件
func (c *Middleware) delete(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("the name can not be empty")
	}

	//判断该中间件是否存在
	position := -1
	for i, v := range c.middlewares {
		if v == name {
			position = i
			break
		}
	}
	//不存在
	if position == -1 {
		return "", fmt.Errorf("the middleware not exist, name: %s", name)
	}

	//删除元素
	c.middlewares = append(c.middlewares[:position], c.middlewares[position+1:]...)

	//落盘中间件列表
	j, _ := json.Marshal(c.middlewares)
	if err := c.uv.ledger.GetBaseDB().Put(keyMiddlewares, j); err != nil {
		return "", err
	}
	return c.get(), nil
}

//获取中间件列表
func (c *Middleware) get() string {
	j, _ := json.Marshal(c.middlewares)
	return string(j)
}

//中间层合约执行 todo 敏感词合约数据双向验证
func (c *Middleware) exec(req *pb.InvokeRPCRequest) error {
	//加锁
	c.locker.Lock()
	defer c.locker.Unlock()

	//打印请求
	//c.prints(req)

	//创建缓存模型
	modelCache, err := xmodel.NewXModelCache(c.uv.GetXModel(), c.uv)
	if err != nil {
		return err
	}

	//创建运行时的上下文配置
	contextConfig := &contract.ContextConfig{
		XMCache:        modelCache,
		Initiator:      req.GetInitiator(),
		AuthRequire:    req.GetAuthRequire(),
		ResourceLimits: contract.MaxLimits,
		Core: contractChainCore{
			Manager: c.uv.aclMgr,
			UtxoVM:  c.uv,
			Ledger:  c.uv.ledger,
		},
		BCName: c.uv.bcname,
	}

	//处理当前交易的所有执行请求
	for _, tmpReq := range req.Requests {

		//检查参数是否有效
		if tmpReq == nil ||
			tmpReq.GetModuleName() == "" &&
				tmpReq.GetContractName() == "" &&
				tmpReq.GetMethodName() == "" ||
			tmpReq.GetModuleName() != "wasm" { //目前只对wasm合约做过滤
			continue
		}

		//限制中间层合约只能由middle命令进行调用
		for _, v := range c.middlewares {
			if v == tmpReq.GetContractName() {
				return fmt.Errorf("middle contract can not invoke in here, you must use middle command, the middle contract is: %s", v)
			}
		}

		//开始执行中间层合约，获取相应虚拟机
		vm, err := c.uv.vmMgr3.GetVM(tmpReq.GetModuleName())
		if err != nil {
			return err
		}

		//交由各个中间件处理
		for _, middleware := range c.middlewares {
			//处理请求的合约
			contextConfig.ContractName = middleware

			//创建上下文，因为是在循环中，所以关闭ctx不能使用defer，需要手动关闭
			ctx, err := vm.NewContext(contextConfig)
			if err != nil {
				//此处可能由于某个中间层合约没有安装导致的错误，进行忽略
				if strings.HasSuffix(err.Error(), "not found") {
					continue
				}
				return err
			}

			//调用该中间件合约，执行其内部的middleware函数，将请求中的参数传入
			res, err := ctx.Invoke(middlewareFunc, tmpReq.GetArgs())
			if err != nil {
				ctx.Release()
				return err
			}
			if res.Status >= 400 {
				ctx.Release()
				return fmt.Errorf("the middle contract invoke fail, the middle contract is: %s,error: %s", middleware, res.Message)
			}
			ctx.Release()
		}
	}
	//全部中间件执行通过
	return nil
}

//打印请求
func (c *Middleware) prints(req *pb.InvokeRPCRequest) {
	fmt.Println("----------------------")
	//fmt.Println("vmMgr3", uv.GetVmMgr3())
	//fmt.Println("aclMgr", uv.GetACLManager())
	//fmt.Println("bcname", uv.GetBcname())
	//fmt.Println("model3", uv.GetXModel())
	//fmt.Println("req", req.String())
	for _, v := range req.GetRequests() {
		fmt.Println("req.v", v.String())
	}
	fmt.Println("----------------------")
}

//管理层执行中间件合约
func (c *Middleware) invoke(action *Proposal) (*pb.InvokeResponse, error) {
	//创建缓存模型
	modelCache, err := xmodel.NewXModelCache(c.uv.GetXModel(), c.uv)
	if err != nil {
		return nil, err
	}

	//创建运行时的上下文配置
	contextConfig := &contract.ContextConfig{
		XMCache:        modelCache,
		Initiator:      action.Vote.Address,
		AuthRequire:    action.Vote.Auth,
		ResourceLimits: contract.MaxLimits,
		Core: contractChainCore{
			Manager: c.uv.aclMgr,
			UtxoVM:  c.uv,
			Ledger:  c.uv.ledger,
		},
		BCName: c.uv.bcname,
	}

	//获取相应虚拟机
	vm, err := c.uv.vmMgr3.GetVM(action.Module)
	if err != nil {
		return nil, err
	}

	//处理请求的合约
	contextConfig.ContractName = action.Contract

	//创建上下文
	ctx, err := vm.NewContext(contextConfig)
	if err != nil {
		return nil, err
	}
	defer ctx.Release()

	//调用该中间件合约
	res, err := ctx.Invoke(action.Method, action.Args)
	if err != nil {
		return nil, err
	}
	if res.Status >= 400 {
		return nil, fmt.Errorf("the middle contract invoke fail, the middle contract is: %s,error: %s", action.Contract, res.Message)
	}

	//创建读写集
	if err = modelCache.WriteTransientBucket(); err != nil {
		return nil, err
	}
	utxoInputs, utxoOutputs := modelCache.GetUtxoRWSets()
	inputs, outputs, err := modelCache.GetRWSets()
	if err != nil {
		return nil, err
	}

	//返回应答，供客户端post
	return &pb.InvokeResponse{
		Inputs:   xmodel.GetTxInputs(inputs),
		Outputs:  xmodel.GetTxOutputs(outputs),
		Response: [][]byte{res.Body},
		//Requests:    requests,
		Responses:   []*pb.ContractResponse{contract.ToPBContractResponse(res)},
		UtxoInputs:  utxoInputs,
		UtxoOutputs: utxoOutputs,
	}, nil
}

//中间层管理 增删改查投票
func (c *Middleware) manage(req *pb.InvokeRPCRequest) (*pb.InvokeResponse, error) {

	//请求中的中间层操作动作
	var action *Proposal

	//提取请求中的投票数据，理论上只会有一条数据
	for _, v := range req.Requests {
		//判断该请求是否是中间层调用
		if v == nil || v.GetModuleName() != "middle" {
			return nil, nil
		}
		//构造当前请求的投票人
		action = &Proposal{
			Vote: &Vote{
				Auth:    req.AuthRequire,
				Address: req.Initiator,
			},
		}
		action.Action = string(v.Args["middle_action"])
		action.Contract = string(v.Args["middle_contract"])
		action.Method = string(v.Args["middle_method"])
		action.UUID = string(v.Args["middle_uuid"])
		action.Module = string(v.Args["middle_module"])
		action.Name = string(v.Args["middle_name"])
		action.Index = string(v.Args["middle_index"])

		//需要使用大整数格式
		if v.Amount == "" {
			v.Amount = "0" //todo 未解之谜,amount会是空字符串
		}
		amount, ok := big.NewInt(0).SetString(v.Amount, 10)
		if !ok {
			return nil, fmt.Errorf("the amount can not conversion to big.Int, the amount is: %s", v.Amount)
		}
		//投票金额
		action.Vote.Amount = amount
		//合约调用参数
		action.Args = v.Args
	}

	//不是中间层的调用则跳出
	if action == nil {
		return nil, nil
	}

	//判断该动作是否可通过
	switch action.Action {
	case "get":
		return invokeResponse(middle.get(), nil)
	case "put", "swap", "del", "vote", "invoke":
		//defer todo 投票池需要落盘
	default:
		return nil, fmt.Errorf("the action is undefined, the action is: %s", action.Action)
	}

	//加锁
	c.mlocker.Lock()
	defer c.mlocker.Unlock()

	//生成投票id
	if action.UUID == "" {
		desc, err := json.Marshal(action)
		if err != nil {
			return nil, err
		}
		id, err := txhash.MakeTxDigestHash(&pb.Transaction{Desc: desc})
		if err != nil {
			return nil, err
		}
		action.UUID = hex.EncodeToString(id)
	}

	//判断该动作请求是否已存在于投票池中
	pool, ok := c.votePool[action.UUID]

	//不存在
	if !ok {
		//投票动作，返回不存在的错误
		if action.Action == "vote" {
			return nil, fmt.Errorf("the vote does not exist, the uuid is: %s", action.UUID)
		}

		//其他动作，插入新记录
		//action.PassCount = big.NewInt(0).Div(c.uv.utxoTotal, big.NewInt(2)) //投票通过的额度
		action.PassCount = big.NewInt(1) //测试时使用的抵押金额
		action.VoteCount = big.NewInt(0)
		action.Votes = make([]*Vote, 0) //加入投票池
		c.votePool[action.UUID] = action

		//返回投票id
		return invokeResponse(fmt.Sprintf("vote_id: %s, pass_amount: %s, count_amount: %s, need_amount: %s",
			action.UUID,
			action.PassCount.String(),
			action.VoteCount.String(),
			action.PassCount.String()), nil)
	}

	//存在则进行投票判断
	//加入投票列表
	pool.Votes = append(pool.Votes, action.Vote)

	//总抵押金额统计
	pool.VoteCount = big.NewInt(0).Add(pool.VoteCount, action.Vote.Amount)
	//判断是否满足抵押需求
	//if pool.VoteCount.Int64() > c.uv.utxoTotal.Int64()/2 {}
	if pool.VoteCount.Cmp(pool.PassCount) >= 0 {

		//解冻抵押金额
		for _, v := range pool.Votes {
			amount, ok := c.vatlist[v.Address]
			if !ok {
				c.vatlist[v.Address] = v.Amount
				continue
			}
			c.vatlist[v.Address] = big.NewInt(0).Add(amount, v.Amount)
		}
		//解冻之后从投票池删除记录
		delete(c.votePool, action.UUID)

		//执行动作
		switch pool.Action {
		case "put":
			//新增中间件
			list, err := c.put(pool.Name, pool.Index)
			return invokeResponse(fmt.Sprintf("execution put, middle contract: %s, index: %s, middle list: %s", pool.Name, pool.Index, list), err)
		case "swap":
			//修改中间件执行顺序
			list, err := c.swap(pool.Name, pool.Index)
			return invokeResponse(fmt.Sprintf("execution swap, middle contract: %s, index: %s, middle list: %s", pool.Name, pool.Index, list), err)
		case "del":
			//删除中间件
			list, err := c.delete(pool.Name)
			return invokeResponse(fmt.Sprintf("execution del, middle contract: %s, middle list: %s", pool.Name, list), err)
		case "invoke":
			//执行中间件合约
			return c.invoke(pool)
		}
	}

	//返回得票数
	return invokeResponse(fmt.Sprintf("vote_id: %s, pass_amount: %s, count_amount: %s, need_amount: %s",
		pool.UUID,
		pool.PassCount.String(),
		pool.VoteCount.String(),
		big.NewInt(0).Sub(pool.PassCount, pool.VoteCount).String()), nil)
}

//构造应答数据结构
func invokeResponse(body string, err error) (*pb.InvokeResponse, error) {
	return &pb.InvokeResponse{
		Responses: []*pb.ContractResponse{
			{Status: 200, Body: []byte(body)},
		},
	}, err
}

//挖矿时构造需要退款的交易
func (c *Middleware) GetVerifiableAutogenTx(blockHeight int64, maxCount int, timestamp int64) ([]*pb.Transaction, error) {
	//是否有待退款数据
	if len(c.vatlist) == 0 {
		return nil, nil
	}
	//加锁
	c.mlocker.Lock()
	defer c.mlocker.Unlock()
	//退款交易列表
	var txs []*pb.Transaction
	//构造退款交易
	for address, amount := range c.vatlist {
		tx, err := c.uv.GenerateAwardTx([]byte(address), amount.String(), []byte{'1'})
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	//清空待退款列表
	c.vatlist = make(map[string]*big.Int)
	return txs, nil
}

func (c *Middleware) GetVATWhiteList() map[string]bool {
	return nil
}
