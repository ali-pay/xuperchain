### 编译合约
cd ../core/contractsdk/go/example/crud/
GOOS=js GOARCH=wasm go build -o crud.wasm crud.go
mv crud.wasm ../../../../../cc/

### 部署合约
cd ../../../../../output/
./xchain-cli wasm deploy --account XC1111111111111111@xuper --cname crud ../cc/crud.wasm --fee 5500000 --runtime go

### 测试功能
./xchain-cli wasm invoke -a '{"key":"key1","value":"value1"}' --method put crud --fee 150000
./xchain-cli wasm query -a '{"key":"key1"}' --method get crud

./xchain-cli wasm invoke -a '{"key":"key2","value":"value2"}' --method put crud --fee 150000
./xchain-cli wasm query -a '{"key":"key2"}' --method get crud

./xchain-cli wasm query -a '{"key":"key"}' --method GetByPrefix crud
./xchain-cli wasm query -a '{"key":"key"}' --method GetMapByPrefix crud

### 更新合约
./xchain-cli wasm upgrade --account XC1111111111111111@xuper --cname crud ../cc/crud.wasm --fee 5500000
```
