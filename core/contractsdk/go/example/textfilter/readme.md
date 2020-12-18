### 创建合约账户
./xchain-cli account new --account 1111111111111111 --fee 1000
./xchain-cli transfer --to XC1111111111111111@xuper --amount 1000000000000

### 编译敏感词合约
cd ../core/contractsdk/go/example/textfilter/
GOOS=js GOARCH=wasm go build
mv textfilter ../../../../../cc/

### 安装敏感词合约
cd ../../../../../output/
./xchain-cli wasm deploy --account XC1111111111111111@xuper --cname text_filter ../cc/textfilter --runtime go --fee 5500000
### 升级敏感词合约（可选）
./xchain-cli wasm upgrade --account XC1111111111111111@xuper --cname text_filter ../cc/textfilter --fee 5500000

### 设置敏感词（建议先从默认的中间件列表中移除，方便直接调用添加多个敏感词）
./xchain-cli middle del text_filter
./xchain-cli middle vote 46d92dc69a64f860e58ccc15d6ceced955163928389da29b6b8bcba3c382c12a 1
#应答：contract response: execution del, middle contract: text_filter, middle list: []

### 移除之后可以作为普通合约直接调用
./xchain-cli wasm invoke -a '{"text":"垃圾"}' --method put text_filter --fee 150000
./xchain-cli wasm invoke -a '{"text":"shit"}' --method put text_filter --fee 150000
./xchain-cli wasm query --method get text_filter

### 设置完成后，重新添加到中间件列表
./xchain-cli middle put text_filter 0
./xchain-cli middle vote 29437514a505f20adcec280c1d07dc40f44b125ca0b06aedc7dea1cce145188c 1
#应答：contract response: execution put, middle contract: text_filter, index: 0, middle list: ["text_filter"]

### 可以查看一下中间件列表
./xchain-cli middle get
#应答：contract response: ["text_filter"]

### 测试敏感词合约执行效果
### 查看crue目录中的readme.md安装crud合约
./xchain-cli wasm invoke -a '{"key":"shit","value":"垃圾"}' --method put crud --fee 150000
#应答：desc = the middle contract invoke fail, the middle contract is: text_filter,error: text_filter is not pass, the param contain: shi
