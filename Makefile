.PHONY: up down down-v logs test loadtest lint lint-fix

up:
	docker-compose up --build -d

down:
	docker-compose down

down-v:
	docker-compose down -v

logs:
	docker-compose logs app
	
test:
	docker-compose -f docker-compose.test.yml up -d --build
	go test -v -count=1 ./integration_test/...
	docker-compose -f docker-compose.test.yml down

loadtest:
	docker-compose -f docker-compose.test.yml up -d --build
	go run ./loadtest/loadtest.go
	docker-compose -f docker-compose.test.yml down

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix