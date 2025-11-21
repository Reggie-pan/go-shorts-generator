build:
\tdocker build -t video-smith .

run:
\tdocker run --rm -it -p 8080:8080 -v ${PWD}/data:/data -v ${PWD}/assets:/assets video-smith

test:
\tcd backend && go test ./...
