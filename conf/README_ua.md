# Файл конфігурації conf/json-scada.json

Конфігураційний файл _json-scada.json_ використовується для інструктажу процесів системи щодо підключення до сервера/бази даних MongoDB.

Після підключення процесу до сервера він завантажить усі необхідні налаштування із сервера.

Цей файл повинен знаходитися в папці installPath/conf/.
Оскільки на одному комп'ютерному сервері ви можете мати кілька систем JSON-SCADA, кожна інсталяція повинна йти по своему шляху.

## Формат файлу

    {
    "nodeName"  : "mainNode",
    "mongoConnectionString": "mongodb://user:password@localhost:27017/json_scada?replicaSet=rs1&authSource=json_scada",
    "mongoDatabaseName": "json_scada",
    "tlsCaPemFile": "c:\\json-scada\\conf\\rootCa.pem",
    "tlsClientPemFile": "c:\\json-scada\\conf\\mongodb.pem",
    "tlsClientPfxFile": "c:\\json-scada\\conf\\mongodb.pfx",
    "tlsClientKeyPassword": "passw0rd",
    "tlsAllowInvalidHostnames": true,
    "tlsAllowChainErrors": true,
    "tlsInsecure": false
    }

* **_nodeName_** - Unique name for a computer installation. This name will be used to match the node configuration in the database. This name is used also to control processes on redundant computers. **Mandatory parameter**.
* **_mongoConnectionString_** - Standard MongoDB connection string pointing to the database server. 
See https://docs.mongodb.com/manual/reference/connection-string/. **Mandatory parameter**
* **_mongoDatabaseName_** - Database name to be accessed in the MongoDB server. **Mandatory parameter**.
* **_tlsCaPemFile_** - Path/Name of the certificate root CA PEM file. **Optional parameter, required for TLS connection**.
* **_tlsClientPemFile_** - Path/Name of the client certificate PEM file. **Optional parameter, required for TLS connection**.
* **_tlsClientPfxFile_** - Path/Name of the client certificate PFX file. **Optional parameter, required for TLS connection**.
* **_tlsClientKeyPassword_** - Password for the PFX file. **Optional parameter, required for TLS connection**.
* **_tlsAllowInvalidHostnames_** - Do not check for the server hostname in certificates. **Optional parameter**.
* **_tlsAllowChainErrors_** - Allows for certificate chain errors. **Optional parameter**.
* **_tlsInsecure_** - Relax other security checks. **Optional parameter**.

Для локального незахищеного підключення до сервера баз даних потрібні лише три перші параметри. ** Не відкривайте незахищені сервери MongoDB для зовнішньої мережі! **

Щоб використовувати авторизацію MongoDB, у рядку підключення вкажіть користувача бази даних та пароль. Цього може бути достатньо для захищених мереж. Для підключення до сервера баз даних за допомогою інструмента MongoDB Compass може знадобитися додати "authSource=json_scada_db_name" в кінець рядка підключення.

Для шифрування підключень до бази даних за допомогою TLS необхідно створити та вказати сертифікати у файлі конфігурації, які можуть бути прийняті сервером MongoDB. Це важливо для з'єднань через Інтернет та ненадійних мереж.

Сервери MongoDB можуть бути Community, Enterprise або Atlas Cloud, всі версії 4.2 або новішої. Інші сервери не тестуються.

Імена хостів, що використовуються у рядку підключення, повинні відповідати іменам хостів для членів набору реплік MongoDB. Ім'я хосту повинно бути доступним через DNS або бути у файлі _ / etc / hosts_ сервера. Набір реплік є обов’язковим для роботи потоків змін і повинен бути створений принаймні з одним членом. Набір реплік із трьох членів настійно рекомендується для надмірності.

## Створення самопідписаних сертифікатів

https://medium.com/@rajanmaharjan/secure-your-mongodb-connections-ssl-tls-92e2addb3c89
