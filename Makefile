all: mp4ff-info mp4ff-nallister mp4ff-pslister mp4ff-wvttlister

mp4ff-info mp4ff-nallister mp4ff-pslister mp4ff-wvttlister:
	go build -ldflags "-X github.com/edgeware/mp4ff/mp4.commitVersion=$$(git describe --tags HEAD) -X github.com/edgeware/mp4ff/mp4.commitDate=$$(git log -1 --format=%ct)" -o out/$@ cmd/$@/main.go

clean:
	rm -f out/*

install: all
	cp out/* $(GOPATH)/bin/

