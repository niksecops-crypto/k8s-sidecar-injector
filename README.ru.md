# Kubernetes Mutating Webhook: Sidecar Injector (Enterprise-Ready)

[Русский](#russian-версия) | [English](#english-version)

---

## English Version

### Project Overview
This project implements a **Mutating Admission Webhook** for Kubernetes in Go. The webhook is designed to automatically inject sidecar containers (e.g., Falco security agents, Fluentd log collectors, or proxy services) into newly created pods within the cluster.

In modern Cloud Native infrastructure, the concept of "automation by default" is critical. This tool ensures that every launched service is protected or monitored without requiring any changes from the application developer.

### Why do you need this? (Use Cases)
As a seasoned engineer, I've highlighted key scenarios where this project is indispensable:
1. **Security Compliance (Falco/Wazuh)**: Automatically deploy threat detection agents into every pod to meet security requirements.
2. **Observability (Prometheus/Fluentd)**: Attach metric exporters or log collectors that must run alongside the main application.
3. **Service Mesh (Custom Linkerd/Istio-like)**: Inject network proxies for traffic management and mTLS encryption.
4. **Secrets Injection**: Attach agents (e.g., HashiCorp Vault Agent) for dynamic secret delivery into containers.

### Technical Stack
- **Go 1.21+**: High performance and minimal resource footprint.
- **Structured Logging (slog)**: Enterprise-grade JSON logging for observability.
- **Prometheus Metrics**: Built-in `/metrics` endpoint for real-time monitoring.
- **Health Checks**: `/healthz` and `/readyz` probes for Kubernetes native lifecycle management.
- **Kubernetes Admission Controller API (v1)**: The industry standard for extending K8s capabilities.
- **JSON Patch (RFC 6902)**: Precise modification of pod manifests without altering original source code.

---

### Getting Started

#### 1. Clone the repository
```bash
git clone https://github.com/niksecops-crypto/k8s-sidecar-injector.git
cd k8s-sidecar-injector
```

#### 2. Prerequisites
Ensure you have the following installed:
- `kubectl` (configured for your cluster)
- `openssl` (for certificate generation)
- `go` (if you plan to modify the code)

### 1. Build the Binary
```bash
# Using Go
go build -o sidecar-injector ./cmd/webhook/main.go

# Using Docker
docker build -t sidecar-injector:latest .
```

---

### Step-by-Step Deployment

#### Step 1: Generate TLS Certificates
Kubernetes Admission Webhooks **must** run over HTTPS. We use self-signed certificates for internal communication.

```bash
chmod +x scripts/gen-certs.sh
./scripts/gen-certs.sh
```
*The script will create a Kubernetes Secret `sidecar-injector-certs` in the `sidecar-injector` namespace and output the `CA_BUNDLE`. Copy it.*

#### Step 2: Configure Webhook Settings
Open `manifests/webhook-config.yaml` and replace `${CA_BUNDLE}` with the value obtained in the previous step.

#### Step 3: Deploy to Cluster
```bash
# Create Namespace
kubectl create namespace sidecar-injector

# Apply manifests
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/deployment.yaml
kubectl apply -f manifests/service.yaml
kubectl apply -f manifests/webhook-config.yaml
```

---

### Validation

To verify the injection is working, run a test pod:

```bash
kubectl run test-pod --image=nginx --restart=Never
```

Check the containers in the pod:
```bash
kubectl get pod test-pod -o jsonpath='{.spec.containers[*].name}'
```
**Expected output:** `nginx security-agent`

---

####### Customization
#### Changing the sidecar image
In `cmd/webhook/main.go`, you can update the template configuration.
В `cmd/webhook/main.go`, определите параметры шаблона.

---

## Russian Версия

### Обзор проекта
Данный проект представляет собой реализацию **Mutating Admission Webhook** для Kubernetes на языке Go. Вебхук предназначен для автоматической инъекции sidecar-контейнеров (например, агентов безопасности Falco, лог-коллекторов Fluentd или прокси-сервисов) во вновь создаваемые поды в кластере.

В современной Cloud Native инфраструктуре концепция "автоматизации по умолчанию" является критически важной. Данный инструмент позволяет гарантировать, что каждый запущенный сервис будет под защитой или мониторингом без участия разработчика приложения.

### Зачем это нужно? (Use Cases)
Как опытный инженер, я выделил основные сценарии, где этот проект незаменим:
1. **Security Compliance (Falco/Wazuh)**: Автоматическое развертывание агентов обнаружения угроз в каждый под для соблюдения требований безопасности.
2. **Observability (Prometheus/Fluentd)**: Подключение экспортеров метрик или сборщиков логов, которые должны работать рядом с основным приложением.
3. **Service Mesh (Custom Linkerd/Istio-like)**: Инъекция сетевых прокси для управления трафиком и шифрования mTLS.
4. **Secrets Injection**: Подключение агентов (например, HashiCorp Vault Agent) для динамической доставки секретов в контейнеры.

### Технический стек
- **Go 1.21+**: Для обеспечения высокой производительности и минимального потребления ресурсов.
- **Структурированное логирование (slog)**: JSON-логирование для удобной интеграции с ELK/Grafana Loki.
- **Prometheus Метрики**: Встроенный эндпоинт `/metrics` для мониторинга в реальном времени.
- **Health Checks**: Проверки `/healthz` и `/readyz` для нативного управления жизненным циклом в K8s.
- **Kubernetes Admission Controller API (v1)**: Стандарт де-факто для расширения возможностей K8s.
- **JSON Patch (RFC 6902)**: Для точечной модификации манифестов подов без изменения их исходного кода.

---

### Как подключиться и начать работу

#### 1. Клонирование репозитория
```bash
git clone https://github.com/niksecops-crypto/k8s-sidecar-injector.git
cd k8s-sidecar-injector
```

#### 2. Подготовка окружения
Убедитесь, что у вас установлены:
- `kubectl` (настроенный на ваш кластер)
- `openssl` (для генерации сертификатов)
- `go` (если планируете вносить изменения в код)

---

### Пошаговое развертывание

#### Шаг 1: Генерация TLS-сертификатов
Kubernetes Admission Webhooks **обязаны** работать через HTTPS. Мы используем самоподписанные сертификаты для внутреннего взаимодействия.

```bash
chmod +x scripts/gen-certs.sh
./scripts/gen-certs.sh
```
*Скрипт создаст Kubernetes Secret `sidecar-injector-certs` в пространстве имен `sidecar-injector` и выведет `CA_BUNDLE`. Скопируйте его.*

#### Шаг 2: Настройка конфигурации вебхука
Откройте `manifests/webhook-config.yaml` и замените `${CA_BUNDLE}` на значение, полученное на предыдущем шаге.

#### Шаг 3: Деплой в кластер
```bash
# Создание Namespace
kubectl create namespace sidecar-injector

# Применение манифестов
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/deployment.yaml
kubectl apply -f manifests/service.yaml
kubectl apply -f manifests/webhook-config.yaml
```

---

### Проверка работы (Validation)

Чтобы убедиться, что инъекция работает, запустите тестовый под:

```bash
kubectl run test-pod --image=nginx --restart=Never
```

Проверьте наличие контейнеров в поде:
```bash
kubectl get pod test-pod -o jsonpath='{.spec.containers[*].name}'
```
**Ожидаемый результат:** `nginx security-agent`

---

### Кастомизация

#### Изменение образа sidecar
В файле `main.go` найдите структуру `sidecar` (строки 95-105). Вы можете изменить образ на любой другой, например:
```go
sidecar := corev1.Container{
    Name:  "log-agent",
    Image: "fluent/fluent-bit:latest",
    // ... ваши аргументы
}
```

---
**Developed specifically for Nik577.**
**Разработано специально для Nik577.**
