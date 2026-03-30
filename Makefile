PROJECT ?= 
PORT ?= 8080

.PHONY: run backend frontend install clean

run: 
	@if [ -z "$(PROJECT)" ]; then \
		echo "Usage: make run PROJECT=/path/to/go/project"; \
		exit 1; \
	fi
	@echo "Starting GoPodView..."
	@echo "  Backend:  http://localhost:$(PORT)"
	@echo "  Frontend: http://localhost:5173"
	@echo "  Project:  $(PROJECT)"
	@trap 'kill 0' EXIT; \
	cd backend && go run main.go --project "$(PROJECT)" --port $(PORT) & \
	cd frontend && npm run dev & \
	wait

backend:
	@if [ -z "$(PROJECT)" ]; then \
		echo "Usage: make backend PROJECT=/path/to/go/project"; \
		exit 1; \
	fi
	cd backend && go run main.go --project "$(PROJECT)" --port $(PORT)

frontend:
	cd frontend && npm run dev

install:
	cd backend && go mod tidy
	cd frontend && npm install

clean:
	cd backend && go clean
	cd frontend && rm -rf node_modules dist
