let vizInstance; // Глобальная переменная для хранения экземпляра Viz.js
let graphData; // Глобальная переменная для хранения данных графа

// Функция для отправки файла на сервер
async function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file);

  try {
    const response = await fetch('/upload', {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      throw new Error('Ошибка загрузки файла');
    }

    // Получаем данные графа с сервера
    const graphResponse = await fetch('/graph');
    if (!graphResponse.ok) {
      throw new Error('Не удалось получить данные графа.');
    }

    graphData = await graphResponse.json(); // Сохраняем данные графа
    renderGraph(); // Рисуем граф после загрузки данных
    fetchAndDisplayMetrics(); // Получаем и отображаем метрики после загрузки графа
  } catch (error) {
    console.error('Ошибка:', error);
    alert(error.message || 'Не удалось загрузить файл или построить граф.');
  }
}

// Функция для получения и отображения метрик
async function fetchAndDisplayMetrics() {
  try {
    const response = await fetch('/metrics');
    if (!response.ok) {
      throw new Error('Не удалось получить отчет по метрикам.');
    }
    const metrics = await response.json();
    displayMetrics(metrics);
  } catch (error) {
    console.error('Ошибка получения метрик:', error);
    // alert(error.message || 'Не удалось получить метрики.');
  }
}

// Функция для отображения метрик на странице
function displayMetrics(metrics) {
  const metricsDisplay = document.getElementById('metrics-display');
  if (!metricsDisplay) {
    console.error('Контейнер для отображения метрик не найден.');
    return;
  }

  // Группируем метрики по категориям
  const groupedMetrics = {};
  if (metrics.metrics) {
    metrics.metrics.forEach(metric => {
      const category = metric.definition.category;
      if (!groupedMetrics[category]) {
        groupedMetrics[category] = [];
      }
      groupedMetrics[category].push(metric);
    });
  }

  let html = '<h3>Общая статистика</h3>';
  html += `<p><strong>Всего экземпляров:</strong> ${metrics.total_process_instances || 0}</p>`;
  html += `<p><strong>Всего событий:</strong> ${metrics.total_events || 0}</p>`;
  html += `<p><strong>Средняя длительность процесса:</strong> ${(metrics.average_process_duration || 0).toFixed(2)} сек.</p>`;
  html += `<p><strong>Медианная длительность процесса:</strong> ${(metrics.median_process_duration || 0).toFixed(2)} сек.</p>`;

  // Вывод сгруппированных метрик
  for (const category in groupedMetrics) {
    html += `<hr><h3>${category}</h3>`;
    groupedMetrics[category].forEach(metric => {
      html += `<div class="metric-card">`;
      html += `<h4>${metric.definition.name}</h4>`;
      html += `<p><strong>Влияние:</strong> ${metric.definition.impact}</p>`;
      html += `<p><strong>Общее значение:</strong> ${metric.total_value.toFixed(2)}</p>`;
      html += `<p><strong>Потерянное время:</strong> ${(metric.total_wasted_duration / 3600).toFixed(2)} ч.</p>`;
      html += `<p><strong>Количество вхождений:</strong> ${metric.count}</p>`;
      if (metric.count > 0) {
        html += `<p><strong>Превышен порог:</strong> ${metric.exceeded ? 'Да' : 'Нет'}</p>`;
        html += '<details><summary><strong>Показать вхождения (' + metric.occurrences.length + ')</strong></summary><ul>';
        metric.occurrences.forEach(occurrence => {
          html += `<li>ID: ${occurrence.InstanceID}, Значение: ${occurrence.Value.toFixed(2)}, Детали: ${occurrence.Details}</li>`;
        });
        html += '</ul></details>';
      }
      html += `</div>`;
    });
  }

  metricsDisplay.innerHTML = html;
}

// Преобразование данных в формат DOT
function convertToDot(data) {
  let dot = 'digraph G {\n';
  dot += '  rankdir=LR;\n'; // Направление графа слева направо
  dot += '  node [shape=rect style=filled];\n'; // Стиль узлов
  dot += '  edge [fontsize=12];\n'; // Стиль ребер

  // Добавление узлов
  data.nodes.forEach(node => {
    const color = node.data.color || '#add8e6'; // Цвет узла
    const label = `${node.data.label} (${node.data.count})`; // Метка узла
    dot += `  "${node.data.id}" [label="${label}" fillcolor="${color}"];\n`;
  });

  // Добавление ребер
  data.edges.forEach(edge => {
    const [events, time] = edge.data.label.split('\n'); // Разделение метки на события и время
    const label = events; // Показываем только количество событий
    dot += `  "${edge.data.from}" -> "${edge.data.to}" [label="${label}"];\n`;
  });

  dot += '}';
  return dot;
}

// Отрисовка графа
async function renderGraph() {
  try {
    if (!graphData) {
      throw new Error('Граф еще не загружен. Загрузите CSV-файл.');
    }

    const powerSlider = document.getElementById('power-slider');
    const powerValue = parseInt(powerSlider.value); // Текущее значение ползунка (0–100%)

    // Получаем диапазон мощности ребер
    const counts = graphData.edges.map(edge => edge.data.count);
    const min = Math.min(...counts);
    const max = Math.max(...counts);

    // Вычисляем пороговое значение мощности
    const threshold = min + ((max - min) * (100 - powerValue)) / 100;

    // Фильтрация ребер по мощности
    const filteredEdges = graphData.edges.filter(edge => edge.data.count >= threshold);

    // Создаем новый объект данных с отфильтрованными ребрами
    const filteredData = {
      nodes: graphData.nodes, // Узлы остаются без изменений
      edges: filteredEdges, // Только отфильтрованные ребра
    };

    const dot = convertToDot(filteredData); // Преобразование данных в формат DOT

    if (!vizInstance) {
      vizInstance = new Viz({
        workerURL: "/js/full.render.js", // Локальный путь
      });
    }

    // Рендеринг DOT в SVG
    const svg = await vizInstance.renderString(dot);
    const graphContainer = document.getElementById('graph');

    // Очищаем контейнер перед новой отрисовкой
    graphContainer.innerHTML = '';

    // Вставляем SVG в DOM
    graphContainer.innerHTML = svg;

    // Инициализация Panzoom
    const panzoomElement = graphContainer.querySelector('svg');
    if (panzoomElement) {
      const panzoom = Panzoom(panzoomElement, {
        maxScale: 5, // Максимальное масштабирование
        minScale: 0.5, // Минимальное масштабирование
        contain: 'outside', // Удерживать содержимое внутри контейнера
      });

      // Включение зума колесиком мыши
      graphContainer.addEventListener('wheel', (e) => {
        e.preventDefault();
        panzoom.zoomWithWheel(e);
      });

      // Центрирование графа
      panzoom.pan(0, 0);
      panzoom.zoom(1);
    }
  } catch (error) {
    console.error('Ошибка рендеринга графа:', error);
    alert(error.message || 'Не удалось отобразить граф');
  }
}

// Функция для скачивания графа в формате PNG
async function downloadPNG() {
  try {
    const graphContainer = document.getElementById('graph');
    const svgElement = graphContainer.querySelector('svg');

    if (!svgElement) {
      throw new Error('Граф еще не построен. Загрузите CSV-файл.');
    }

    // Клонируем SVG для корректного экспорта
    const clone = svgElement.cloneNode(true);
    const serializer = new XMLSerializer();
    const svgString = serializer.serializeToString(clone);

    // Создаем Blob из SVG
    const blob = new Blob([svgString], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);

    // Создаем временный canvas для конвертации SVG в PNG
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');

    // Получаем реальные размеры SVG
    const svgWidth = svgElement.width.baseVal.value || svgElement.clientWidth;
    const svgHeight = svgElement.height.baseVal.value || svgElement.clientHeight;

    // Устанавливаем размеры canvas с учетом DPR (Device Pixel Ratio)
    const dpr = window.devicePixelRatio || 1; // Плотность пикселей устройства
    canvas.width = svgWidth * dpr;
    canvas.height = svgHeight * dpr;

    // Масштабируем контекст canvas для повышения качества
    ctx.scale(dpr, dpr);

    // Создаем изображение из SVG
    const img = new Image();
    img.onload = () => {
      ctx.drawImage(img, 0, 0, svgWidth, svgHeight);

      // Конвертируем canvas в PNG
      canvas.toBlob(
        (blob) => {
          const pngUrl = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = pngUrl;
          a.download = 'graph.png';
          document.body.appendChild(a);
          a.click();
          a.remove();
          URL.revokeObjectURL(pngUrl);
        },
        'image/png',
        1.0 // Настройка качества PNG (1.0 — максимальное качество)
      );
    };

    img.src = url;
  } catch (error) {
    console.error('Ошибка скачивания PNG:', error);
    alert(error.message || 'Не удалось скачать PNG');
  }
}

// Функция для очистки графа
async function clearGraph() {
  try {
    const response = await fetch('/clear', {
      method: 'POST',
    });

    if (!response.ok) {
      throw new Error('Не удалось очистить граф.');
    }

    // Очищаем данные графа на фронте
    graphData = null;

    // Очищаем контейнер графа
    const graphContainer = document.getElementById('graph');
    graphContainer.innerHTML = '';

    // Деактивируем кнопку скачивания
    document.getElementById('download-btn').disabled = true;

  } catch (error) {
    console.error('Ошибка очистки графа:', error);
    alert(error.message || 'Не удалось очистить граф.');
  }
}

// Инициализация
document.addEventListener('DOMContentLoaded', () => {
  const fileInput = document.getElementById('file-input');
  const uploadBtn = document.getElementById('upload-btn');
  const downloadBtn = document.getElementById('download-btn');
  const exportMetricsBtn = document.getElementById('export-metrics-btn');
  const powerSlider = document.getElementById('power-slider');
  const powerValue = document.getElementById('power-value');
  const clearBtn = document.getElementById('clear-btn');

  if (!fileInput || !uploadBtn || !downloadBtn || !powerSlider || !powerValue || !clearBtn) {
    console.error('Один или несколько элементов DOM не найдены.');
    return;
  }

  // Клик на кнопку "Загрузить файл"
  uploadBtn.addEventListener('click', () => {
    fileInput.click(); // Программно вызываем выбор файла
  });

  // Обработка выбора файла
  fileInput.addEventListener('change', () => {
    const file = fileInput.files[0];
    if (file) {
      uploadFile(file); // Автоматическая загрузка файла при выборе
      downloadBtn.disabled = false; // Активируем кнопку скачивания
      exportMetricsBtn.disabled = false; // Активируем кнопку экспорта метрик
    } else {
      alert('Выберите файл для загрузки.');
    }
  });

  // Клик на кнопку "Скачать PNG"
  downloadBtn.addEventListener('click', downloadPNG);

  // Клик на кнопку "Экспорт метрик (JSON)"
  exportMetricsBtn.addEventListener('click', async () => {
    try {
      const response = await fetch('/metrics');
      if (!response.ok) {
        throw new Error('Не удалось получить отчет по метрикам для экспорта.');
      }
      const metrics = await response.json();
      
      // Создаем Blob, чтобы гарантировать правильную кодировку
      const blob = new Blob([JSON.stringify(metrics, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);

      const downloadAnchorNode = document.createElement('a');
      downloadAnchorNode.href = url;
      downloadAnchorNode.download = 'metrics_report.json';
      document.body.appendChild(downloadAnchorNode);
      downloadAnchorNode.click();
      document.body.removeChild(downloadAnchorNode);
      URL.revokeObjectURL(url); // Освобождаем память

    } catch (error) {
      console.error('Ошибка экспорта метрик:', error);
      alert(error.message || 'Не удалось экспортировать метрики.');
    }
  });

  // Клик на кнопку "Очистить граф"
  clearBtn.addEventListener('click', clearGraph);

  // Изменение значения ползунка
  powerSlider.addEventListener('input', () => {
    powerValue.textContent = `${powerSlider.value}%`; // Обновляем отображаемое значение
    renderGraph(); // Перерисовываем граф при изменении ползунка
  });
});