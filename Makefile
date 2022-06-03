bin:
	go build -o ododev main.go 

install: bin
	cp ododev ${HOME}/bin
