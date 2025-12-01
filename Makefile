build:
	docker build -t goshortsgenerator .

run:
	docker run --rm -it -p 8080:8080 -v ${PWD}/data:/data -v ${PWD}/assets:/assets goshortsgenerator

test:
	cd backend && go test ./...
