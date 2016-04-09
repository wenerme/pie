# Pie

操控 Raspberry 的小工具, 实用 JavaScript 和 Golang 绑定实现

## 安装

```
go get github.com/wenerme/pie
```

## 基本操作
具体接口请参考 `gpio/face.go`

```
gpio.Pin(10).Read() // 获取 Pin 10 状态
gpio.Pin(10).Low() // 设置 Pin 10 为关闭状态
```


