## Сервер даних HTTP/JSON у реальному часі

Цей модуль NodeJS/Express може обслуговувати дані реального часу JSON-SCADA для веб-інтерфейсу.

Він також обслуговуе HTML-файли з папки src/htdocs.

Можна отримати доступ до Grafana по шляху "/grafana", налаштувавши змінну середовища _JS_GRAFANA_SERVER_.

Рекомендується застосовувати зворотний проксі-сервер (Nginx) поверх цієї служби, щоб надійно обслуговувати клієнта у зовнішніх мережах. Для найкращої масштабованості статичні файли повинні подаватися безпосередньо через Nginx або Apache, перенаправляючи _/grafana_ на сервер Grafana та _/Invoke_ до цієї служби Node.js.

### Приклад конфігурації Nginx як зворотного проксі

# data API
    location /Invoke/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:8080/Invoke/;
    }

    # Grafana server
    location /grafana/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:3000/;
    }

    # Веб-доступ супервізора, за бажанням
    location /supervisor/ {
        proxy_set_header   X-Forwarded-For $remote_addr;
        proxy_set_header   Host $http_host;
        proxy_pass         http://127.0.0.1:9000/;
    }

    # Static files
    location / {
        root /home/username/json-scada/src/htdocs;
    }

## API служб даних

Access point : /Invoke/

Натхнення надійшло з додатка OPC https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference

Read Service Request
Write Service Request
Read History Service Request
Request Unique Attributes Value

## Змінні середовища

* _**JS_IP_BIND**_ [String] - IP-адреса для прослуховування сервером. Використовуйте "0.0.0.0" для прослуховування на всіх інтерфейсах. **Необов’язковий аргумент, за замовчуванням = "localhost" (лише локальний хост)**.
* _**JS_HTTP_PORT**_ [Integer] - HTTP-порт для прослуховування сервера. **Необов’язковий аргумент, за замовчуванням = 8080**.
* _**JS_GRAFANA_SERVER**_ [Integer] - URL-адреса HTTP на сервер Grafana (для зворотного проксі на /grafana). **Необов’язковий аргумент, за замовчуванням = "http://127.0.0.1:3000"**.