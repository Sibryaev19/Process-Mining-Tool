#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Подготовка к демонстрации Process Mining ===${NC}"

# Check if port 8085 is already in use
if lsof -Pi :8085 -sTCP:LISTEN -t >/dev/null ; then
    echo "Внимание: Порт 8085 уже занят. Убедитесь, что приложение не запущено дважды."
else
    echo "Порт 8085 свободен."
fi

echo -e "${GREEN}Запуск сервера...${NC}"
echo "Датасет для демонстрации находится здесь: $(pwd)/datasets/largest_dataset.csv"
echo "Через 5 секунд откроется браузер. Если нет, перейдите по ссылке: http://localhost:8085"

# Run the server in the background
# Using the command from Makefile
go run ./cmd/app/main.go serve &
SERVER_PID=$!

# Wait for server to initialize
sleep 5

# Open browser
open "http://localhost:8085"

echo -e "${GREEN}Сервер работает (PID: $SERVER_PID).${NC}"
echo -e "${BLUE}Для загрузки файла выберите: datasets/largest_dataset.csv${NC}"
echo "Нажмите Ctrl+C для остановки сервера и выхода."

# Handle cleanup on exit
trap "kill $SERVER_PID" EXIT

# Wait for user to kill the script
wait $SERVER_PID
