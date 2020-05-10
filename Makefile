services-up:
	@echo "$$(date) === RUN DEVELOPMENT ENVIRONMENT (AUTO BUILD)"
	@echo "         === Running CHOWN so that docker data can be overwritten"
	@sudo chown -R $$(id -u):$$(id -g) .docker
	@echo "         === Docker UP!"
	@docker-compose up --build

services-down:
	@docker-compose down