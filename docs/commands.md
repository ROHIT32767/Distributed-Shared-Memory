
### Command to run master
```bash
cd Distributed\ Shared\ Memory/; cd src; go run master/main.go
```
### Command to run slave
```bash
cd Distributed\ Shared\ Memory/; cd src; go run slave/main.go
```

### Command to run client
```bash
cd Distributed\ Shared\ Memory/; cd src; go run test_harness/main.go -slaves 5 -clients 18
```