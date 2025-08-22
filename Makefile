.PHONY: all clean wsh wcp

all: wsh wcp

wsh:
	go build -o wsh/wsh wsh/main.go

wcp:
	go build -o wcp/wcp wcp/main.go

clean:
	rm -f wsh/wsh wcp/wcp

install: all
	cp wsh/wsh /usr/local/bin/
	cp wcp/wcp /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/wsh /usr/local/bin/wcp
