## Сервер даних HTTP / JSON у реальному часі 

Цей модуль NodeJS / Express може обслуговувати дані реального часу JSON-SCADA для веб-інтерфейсу. 

Він також може серверувати HTML-файли з папки src / htdocs. 

Можна отримати доступ до Grafana по шляху "/ grafana", налаштувавши змінну середовища _JS_GRAFANA_SERVER_. 

Рекомендується застосовувати зворотний проксі-сервер (Nginx) поверх цієї служби, щоб надійно обслуговувати клієнта у зовнішніх мережах. Для найкращої масштабованості статичні файли повинні подаватися безпосередньо через Nginx або Apache, перенаправляючи _ / grafana_ на сервер Grafana та _ / Invoke_ до цієї служби Node.js. 

### Приклад конфігурації Nginx як зворотного проксі 

    # data 
    location location / / { 
        Invoke proxy_set_header X-Forwarded-For $ remote_addr;
        proxy_set_header Хост $ http_host; 
        proxy_pass http://127.0.0.1:8080/Invoke/; 
    } 

    # Розташування сервера Grafana 
    / grafana / { 
        proxy_set_header X-Forwarded-For $ remote_addr; 
        proxy_set_header Хост $ http_host; 
        proxy_pass http://127.0.0.1:3000/; 
    } 

    # Веб-доступ супервізора, за бажанням 
    розташування / супервізор / { 
        proxy_set_header X-Forwarded-For $ remote_addr; 
        proxy_set_header Хост $ http_host; 
        proxy_pass http://127.0.0.1:9000/; 
    } 

    # 
    Розташування статичних файлів / { 
        root / home / username / json-scada / src / htdocs; 
    } 

## 

Точка доступу API служб даних : / Invoke /
 
Натхненний наведеним нижче матеріалом OPC-UA: 

* https://prototyping.opcfoundation.org 
* https://www.youtube.com/watch?v=fiuamY0DzLM 
* https: //reference.opcfoundation. org 
* https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference 

### Сервіс читання 

Ця служба використовується для зчитування значень реального часу з сервера MongoDB. Необхідно вказати вузли (точки даних), які будуть зчитуватися цифровим ключем __id_ або рядковим ключем _tag_. 

Довідкова документація https://reference.opcfoundation.org/Core/docs/Part4/5.10.2/. 

Запит на читання: викликати метод Post для точки доступу / Invoke із такою структурою JSON: 

    {
        "ServiceId": 629,  
        "Body": {
            "RequestHeader": { 
            "Timestamp": "2020-09-17T12: 40: 47.373Z", 
            "RequestHandle": 54027318, 
            "TimeoutHint": 1500, 
            "ReturnDiagnostics": 2, 
            "AuthenticationToken": null 
            }, 
            "MaxAge": 0, 
            "TimestampsToReturn": 2, 
            "NodesToRead": [ 
            { 
                "NodeId": {"IdType": 0, "Id": 6620, "Простір імен": 2}, 
                "AttributeId": 13 
            },
            { 
                "NodeId": {"IdType": 1, "Id": "KAW2AL-27XCBR5231", "Простір імен": 2}, 
                "AttributeId" :13 
            } 
            ] 
        } 
    } 

Приклад відповіді:
 
    { 
        "NamespaceUris": [ 
            "urn: opcf-apps-01: UA: Quickstarts: ReferenceServer", 
            "http://opcfoundation.org/Quickstarts/ReferenceApplications", 
            "http: // opcfoundation .org / UA / Diagnostics " 
        ], 
        " ServerUris ": [], 
        " ServiceId ": 632, 
        " Body ": { 
            " ResponseHeader ": { 
            " RequestHandle ": 54027318, 
            " Timestamp ":" 2020-09-17T12: 40 : 48.305Z ", 
            " Сервісна діагностика ":{"LocalizedText": 0}, 
            "StringTable": [ 
                "Добре",
                "Операція завершена успішно.", 
                "SourceTimestamp": "2020-09-17T12: 40: 36.168Z"Операція завершена успішно. ", 
                " Час запиту: 12 мс " 
            ],
            "ServiceResult": 0 
            }, 
            "Результати": [ 
            { 
                "StatusCode": 0, 
                "NodeId": {"IdType": 1, "Id": "KAW2BC1 - RBLK", "Простір імен": 2}, 
                "Значення ": {" Тип ": 1," Тіло ": хибне," Якість ": 0}, 
                " _Properties ": { 
                    " _id ": 6620, 
                    " valueString ":" НОРМАЛЬНЕ ", 
                    " занепокоєне ": правда, 
                    " анотація " : "", 
                    "origin": "під наглядом"
                }, 
            { 
                "StatusCode": 0,
            }, 
                "NodeId": {"IdType": 1, "Id": "KAW2AL-27XCBR5231", "Простір імен": 2}, 
                "Значення": {"Тип": 1, "Тіло": true, "Якість" : 0}, 
                "_Properties": { 
                    "_id": 3279, 
                    "valueString": "ON", 
                    "alarmed": false, 
                    "annotation": "", 
                    "origin": "supervisor" 
                }, 
                "SourceTimestamp": " 2020-09-14T17: 51: 12.547Z "
            } 
            ] 
        } 
    } 

Об’єкт стандарту __Properties_, що не відповідає OPC-UA, з відповіді має розширені атрибути для точки.

### Служба запису 

Ця служба використовується для надсилання команд з веб-інтерфейсу на сервер. 

Довідкова документація: https://reference.opcfoundation.org/Core/docs/Part4/5.10.4/. 

Запит на запит: викликайте метод Post для точки доступу / Invoke із такою структурою JSON: 

    { 
        "ServiceId": 671, 
        "Body": { 
        "RequestHeader": { 
            "Timestamp": "2020-09-17T13: 35: 30.186 Z ", 
            " RequestHandle ": 94046531, 
            " TimeoutHint ": 1500, 
            " ReturnDiagnostics ": 2, 
            " AuthenticationToken ": null 
        }, 
        " NodesToWrite ": 
            "
            "AttributeId": 13, 
            "Value": {"Type": 11, "Body": 1} 
            } 
        ] 
    } 


Відповідь: 

    { 
        "NamespaceUris": [ 
            "urn: opcf-apps-01: UA: Quickstarts: ReferenceServer", 
            "http://opcfoundation.org/Quickstarts/ReferenceApplications", 
            "http://opcfoundation.org/UA/Diagnostics" 
        ], 
        "ServerUris": [], 
        "ServiceId": 674, 
        "Body": { 
            "ResponseHeader" : {
            "ServiceDiagnostics": {"LocalizedText": 0}, 
            "StringTable": [],ResponseHeader ": { 
            " RequestHandle ": 94046531,
            "Timestamp": "2020-09-17T13: 35: 31.115Z", 
            "ServiceResult": 0 
            }, 
        "Results": [0], 
        "_CommandHandles": ["5f6366233fabf37f071097e9"] 
        } 
    } 

Далі, використовуйте запити на читання (з AttributeId: 12) для моніторингу зворотного зв'язку з командами, використовуючи значення __CommandHandles_. 

    { 
        "ServiceId": 629, 
        "Body": { 
        "RequestHeader": { 
            "Timestamp": "2020-09-17T13: 35: 30.396Z", 
            "RequestHandle": 78655446, 
            "TimeoutHint": 1250, 
            "ReturnDiagnostics":
            "NodeId": {"IdType": 0, "Id": 64083, "Простір імен": 2}, 
            "AttributeId": 12, 
            "ClientHandle": "5f6366233fabf37f071097e9" 
            } 
        ] 
        } 
    } 

Відповідь, визначена в https: // посилання .opcfoundation.org / Core / docs / Part4 / 7.20.2 / 

    { 
        " SpacepaceUris ": [ 
            "urn: opcf-apps-01: UA: Quickstarts: ReferenceServer", 
            "http://opcfoundation.org/Quickstarts/ReferenceApplications" , 
            "http://opcfoundation.org/UA/Diagnostics" 
        ], 
        "ServerUris": [], 
        "ServiceId ": 809,  
        " Body ": {
            " ResponseHeader ": {
            "RequestHandle": 78655446, 
            " Клейма часу": "2020-09-17T13: 35: 31.325Z", 
            "ServiceDiagnostics": {"LocalizedText": 0}, 
            "StringTable": [], 
            "ServiceResult": 0 
            }, 
            " MonitoredItems ": [ 
            { 
                " ClientHandle ":" 5f6366233fabf37f071097e9 ", 
                " Value ": {" Value ": 1," StatusCode ": 2159149056}, 
                " NodeId ": { 
                " IdType ": 1, 
                " Id ":" KAW2TR2-0XCBR5206 ---- К ", 
                "Простір імен ": 2 
                } 
            } 
    } 
            ]
        } 

Коли StatusCode для команди 0 (Добре), команда була визнана нормальною. 

### Читання запиту на послугу історії 
### запит Значення унікальних атрибутів 

## Змінні середовища 

* _ ** JS_IP_BIND ** _ [рядок] - IP-адреса для прослуховування сервером. Використовуйте "0.0.0.0" для прослуховування на всіх інтерфейсах. ** За замовчуванням = "localhost" (лише локальний хост) **. 
* _ ** JS_HTTP_PORT ** _ [Ціле число] - HTTP-порт для прослуховування сервера. ** За замовчуванням = 8080 **. 
* _ ** JS_GRAFANA_SERVER ** _ [Ціле число] - HTTP-URL на сервер Grafana (для зворотного проксі на / grafana). ** За замовчуванням = "http://127.0.0.1:3000" **. 
* _ ** JS_CONFIG_FILE ** _ [Рядок] - ім'я файлу конфігурації JSON SCADA. ** За замовчуванням = "../../ conf / json-scada.json" **. 

## Аргументи командного рядка

Цей процес не має аргументів командного рядка.
