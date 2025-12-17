import csv
import random
import datetime

class LogGeneratorConfig:
    def __init__(self, output_file: str, num_instances: int, max_events: int, 
                 add_self_loops: int, add_ping_pongs: int, add_anomalies: int, 
                 add_errors: int, incomplete_rate: float):
        self.output_file = output_file
        self.num_instances = num_instances
        self.max_events = max_events
        self.add_self_loops = add_self_loops
        self.add_ping_pongs = add_ping_pongs
        self.add_anomalies = add_anomalies
        self.add_errors = add_errors
        self.incomplete_rate = incomplete_rate

class Event:
    def __init__(self, case_id: str, timestamp: datetime.datetime, activity: str, result: str):
        self.case_id = case_id
        self.timestamp = timestamp
        self.activity = activity
        self.result = result

def inject_self_loop(events: list[Event]) -> list[Event]:
    if len(events) < 2: 
        return events
    insert_index = random.randint(1, len(events) - 1)
    loop_event = Event(events[insert_index - 1].case_id, 
                       events[insert_index - 1].timestamp + datetime.timedelta(minutes=1), 
                       events[insert_index - 1].activity, 
                       events[insert_index - 1].result)
    return events[:insert_index] + [loop_event] + events[insert_index:]

def inject_ping_pong(events: list[Event]) -> list[Event]:
    if len(events) < 2: 
        return events
    insert_index = random.randint(1, len(events) - 1)
    ping = events[insert_index - 1]
    pong = events[insert_index]
    
    pong.timestamp = ping.timestamp + datetime.timedelta(minutes=1)
    ping.timestamp = pong.timestamp + datetime.timedelta(minutes=1)
    
    return events[:insert_index] + [pong, ping] + events[insert_index+1:]

def inject_anomaly(events: list[Event]) -> list[Event]:
    if len(events) < 2: 
        return events
    anomaly_index = random.randint(1, len(events) - 1)
    events[anomaly_index].timestamp = events[anomaly_index - 1].timestamp + datetime.timedelta(minutes=random.randint(60, 180))
    return events

def inject_error(events: list[Event]) -> list[Event]:
    if not events: 
        return events
    error_index = random.randint(0, len(events) - 1)
    events[error_index].result = "error"
    return events

def generate_instance(case_id: str, start_time: datetime.datetime, config: LogGeneratorConfig) -> list[Event]:
    events = []
    num_events = random.randint(3, config.max_events)
    current_time = start_time

    # Добавляем начальное событие
    events.append(Event(case_id, current_time, "Начало процесса", "success"))

    for j in range(1, num_events):
        activity_name = f"Этап {chr(ord('A') + random.randint(0, 4))}"
        current_time += datetime.timedelta(minutes=random.randint(5, 15))
        events.append(Event(case_id, current_time, activity_name, "success"))

    # Внедряем отклонения
    if config.add_self_loops > 0 and random.random() < 0.5:
        events = inject_self_loop(events)
        config.add_self_loops -= 1
    if config.add_ping_pongs > 0 and random.random() < 0.5:
        events = inject_ping_pong(events)
        config.add_ping_pongs -= 1
    if config.add_anomalies > 0 and random.random() < 0.5:
        events = inject_anomaly(events)
        config.add_anomalies -= 1
    if config.add_errors > 0 and random.random() < 0.5:
        events = inject_error(events)
        config.add_errors -= 1

    # Добавляем конечное событие (или пропускаем его)
    if random.random() > config.incomplete_rate:
        current_time += datetime.timedelta(minutes=random.randint(5, 15))
        events.append(Event(case_id, current_time, "Конец", "success"))

    return events

def generate_log(config: LogGeneratorConfig):
    try:
        with open(config.output_file, 'w', newline='', encoding='utf-8') as file:
            writer = csv.writer(file)
            writer.writerow(["case_id", "timestamp", "activity", "result"])

            start_time = datetime.datetime.now()

            for i in range(config.num_instances):
                case_id = f"case_{i + 1}"
                events = generate_instance(case_id, start_time, config)
                start_time += datetime.timedelta(minutes=random.randint(0, 60)) # Сдвигаем время для следующего экземпляра

                for event in events:
                    writer.writerow([event.case_id, event.timestamp.isoformat(), event.activity, event.result])
        print(f"Лог успешно сгенерирован в {config.output_file}")
    except IOError as e:
        print(f"Ошибка при создании или записи файла: {e}")

if __name__ == "__main__":
    # Пример использования:
    config = LogGeneratorConfig(
        output_file="generated_log.csv",
        num_instances=10,
        max_events=10,
        add_self_loops=2,
        add_ping_pongs=2,
        add_anomalies=2,
        add_errors=2,
        incomplete_rate=0.1
    )
    generate_log(config)