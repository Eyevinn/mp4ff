build:
	go build -ldflags "-X github.com/Eyevinn/mp4ff/mp4.commitVersion=$$(git describe --tags HEAD) -X github.com/Eyevinn/mp4ff/mp4.commitDate=$$(git log -1 --format=%ct)" .

install:
	go install -ldflags "-X github.com/Eyevinn/mp4ff/mp4.commitVersion=$$(git describe --tags HEAD) -X github.com/Eyevinn/mp4ff/mp4.commitDate=$$(git log -1 --format=%ct)" .

linux-build:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.commitVersion=$$(git describe --tags HEAD) -X main.commitDate=$$(git log -1 --format=%ct)" 

