# Kubernetes Mutating Webhook: Sidecar Injector (Enterprise-Ready)

[Русский](README.ru.md) | [English](README.md)

---

## Russian Версия

### Обзор проекта
Данный проект представляет собой реализацию **Mutating Admission Webhook** для Kubernetes на языке Go. Вебхук предназначен для автоматической инъекции sidecar-контейнеров (например, агентов безопасности Falco, лог-коллекторов Fluentd или прокси-сервисов) во вновь создаваемые поды.

Этот проект значительно превосходит стандартные инжекторы благодаря использованию **Kubernetes Native Sidecars (initContainers + RestartPolicy: Always)**, добавленных в Kubernetes 1.28+. Это гарантирует идеальный порядок запуска и остановки ваших сайдкаров!

Подробнее о том, почему это важно, читайте в нашей [Wiki по Native Sidecars](docs/wiki/native-sidecars.md).

### Ключевые Enterprise Фичи
- **Нативные Сайдкары**: Инъекция происходит в `initContainers`, а не в `containers`. Это гарантирует, что сайдкары стартуют первыми, а завершаются последними.
- **Горячая перезагрузка конфига (Hot-Reload)**: Изменяйте список внедряемых сайдкаров через ConfigMap без перезагрузки самого вебхука (SIGHUP reload без даунтайма).
- **Строгая валидация**: Защита от опечаток, проверка дубликатов имен сайдкаров до применения конфигурации.
- **Prometheus Метрики**: Встроенный мониторинг (`sidecar_injector_mutations_total` и `sidecar_injector_mutation_duration_seconds`) доступен на порту `:8443/metrics`.
- **Безопасность (Restricted Pod Security)**: Процесс запускается без root-прав, с read-only файловой системой и сбросом всех capabilities (`drop: ["ALL"]`).
- **Auto-TLS Fallback**: Нативная поддержка `cert-manager` для продакшена, но при его отсутствии вебхук автоматически генерирует самоподписанные сертификаты для локальной разработки.

### Технический стек
- **Go 1.21+**: Для обеспечения высокой производительности и нулевых сторонних зависимостей в ядре.
- **Helm 3**: Для сборки и развертывания проекта в кластер.
- **JSON Patch (RFC 6902)**: Стандарт для точечной модификации манифестов.

---

### Как подключиться и начать работу

#### 1. Клонирование репозитория
```bash
git clone https://github.com/niksecops-crypto/k8s-sidecar-injector.git
cd k8s-sidecar-injector
```

#### 2. Деплой через Helm (Рекомендуется)
```bash
helm upgrade --install k8s-sidecar-injector ./deploy/helm/k8s-sidecar-injector \
    --namespace sidecar-injector --create-namespace
```

Если у вас не установлен `cert-manager`, Helm-чарт автоматически передаст `AUTO_GENERATE_CERT=true` в деплоймент, и вебхук сам сгенерирует для себя самоподписанные сертификаты во временном томе (`emptyDir`)!

### Проверка работы (Validation)

Чтобы убедиться, что инъекция работает, запустите тестовый под с аннотацией `sidecar-injector.io/inject: "true"`:

```bash
kubectl run test-pod --image=nginx --restart=Never --labels="sidecar-injector.io/inject=true" --annotations="sidecar-injector.io/inject=true"
```

Проверьте список `initContainers` в поде, чтобы увидеть добавленный Нативный Сайдкар:
```bash
kubectl get pod test-pod -o jsonpath='{.spec.initContainers[*].name}'
```
**Ожидаемый результат:** `security-agent` (или те сайдкары, которые вы настроили в `values.yaml`).

---

### Кастомизация
Настройте сайдкары для инъекции через Helm `values.yaml` (`deploy/helm/k8s-sidecar-injector/values.yaml`).

```yaml
sidecars:
  - name: "my-sidecar"
    image: "my-image:latest"
    # ... любые стандартные поля corev1.Container
```

---
**Разработано специально для Nik577.**
