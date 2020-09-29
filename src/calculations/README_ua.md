# {json:scada} calculations.go

Процес _calculations_ відповідає за виконання заздалегідь визначених (скомпільованих) формул над посилками для оновлення значення обчислених показників. Період розрахункового циклу за замовчуванням - 2 секунди. Повний набір розрахунків показників перераховується на кожному циклі.

Цей спосіб розрахунку є зручним та ефективним. Він не споживає потоки змін і записує лише змінені значення. Недоліком цього методу є затримка в порядку періоду циклу. Період розрахунку можна змінити, якщо це необхідно, але слід розумно передбачити трохи вільного часу.

Якщо необхідно обчислити значення з дуже низькою затримкою, необхідно створити новий процес користувацького потоку змін, який буде перераховувати значення відразу після кожної зміни посилки. Для цього слід надати власний код формули (число понад 100000).

Розрахована точка повинна бути визначена у колекції _realtimeData_.
Це повинно бути

* Позначте _origin="calculated" _.
* Визначте код _formula_.
* Визначте його посилки (array of point key , __id_ посилок).


```
    {
    "_id": 6240,
    "description": "KNH2~TL1 KYU2 230kV~Apparent Power-Calc",
    "formula": 3,
    "group1": "KNH2",
    "group2": "TL1 KYU2 230kV",
    "group3": "",
    "origin": "calculated",
    "parcels": [28973, 28974],
    ...
    "value": 118.39308345399448
    }

```

Доступні формули наведені нижче.

* Formula **1** - Current based on Active/Reactive powers and voltage. (1000/SQRT(3))*SQRT(P1^2+P2^2)/P3.
* Formula **2** - P1 / SQRT((P1^2) + (P2^2)).
* Formula **3** - Apparent power based on Active/Reactive powers. SQRT((P1^2) + (P2^2)).
* Formula **4** - Sum. P1+P2+P3+...+Pn.
* Formula **5** - SQRT(P1).
* Formula **6** - Logical AND (P1 and P2 and ... and Pn).
* Formula **7** - Logical OR (P1 or P2 or ... or Pn).
* Formula **8** - Timer (Unix time n seconds).
* Formula **9** - Apparent power based on amps/kV. P1 * P2 * SQRT(3) / 1000.
* Formula **10** - Negative sum. -(P1+P2+P3+...+Pn).
* Formula **11** - Reserved.
* Formula **12** - Reserved.
* Formula **13** - Reserved.
* Formula **14** - Reserved.
* Formula **15** - Difference of 2. P1 - P2.
* Formula **16** - Difference of 3. P1 - P2 - P3.
* Formula **17** - Difference of 4. P1 - P2 - P3 - P4.
* Formula **18** - Difference of 6. P1 - P2 - P3 - P4 - P5 - P6.
* Formula **19** - P1 + P2 - P3.
* Formula **20** - P1 + P2 + P3 - P4 - P5 - P6.
* Formula **21** - P1 + P2 + P3 + P4 - P5 - P6 - P7 - P8 - P9.
* Formula **22** - P1 + P2 + P3 + P4 + P5 + P6 - P7 - P8 - P9.
* Formula **23** - P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 + P10 - P11.
* Formula **24** - P1 + P2 + P3 - P4 - P5 - P6 - P7 - P8 - P9.
* Formula **25** - Reserved.
* Formula **26** - RES=P2;if (abs(P1)>1.4) RES=0; if (abs(P1)<=0.5) RES=1.
* Formula **27-49** - Reserved.
* Formula **50** - DIGITAL/ANALOG CHOICE (pick the first ok value from n parcels).
* Formula **51** - DIGITAL/ANALOG CHOICE (pick the first ok value from n parcels).
* Formula **52** - Any ok? (1 if any parcel is ok from n parcels).
* Formula **53** - MAX SPAN (difference between max / min from n parcel values).
* Formula **54** - Double point from 2 single OFF / ON = OFF,  ON / OFF = ON, equal values = bad,transient.
* Formula **55** - Division. P1/P2.
* Formula **56-199** - Reserved.
* Formula **200** - P1-P2-P3-P4-P5-P6-P7-P8.
* Formula **201** - P1+P2+P3+P4+P5+P6+P7+P8-P9-P10-P11.
* Formula **202** - ( P1 * 60 ) + P2.
* Formula **203** - Choose from 2 measures P1 !=0 THEN P3 ELSE P2.
* Formula **204** - Divide by 2. P1/2.
* Formula **205** - P1+P2-P3-P4-P5.
* Formula **206** - P1-P2-P3-P4-P5-P6-P7-P8-P9.
* Formula **207** - P1+P2-P3-P4-P5-P6.
* Formula **208** - P1-P2-P3-P4-P5.
* Formula **209** - P1+P2-P3-P4-P5-P6-P7-P8-P9.
* Formula **210** - P1-P2-P3-P4-P5-P6-P7.
* Formula **211** - P1+P2+P3+P4+P5-P6-P7-P8-P9.
* Formula **212** - Reserved.
* Formula **213** - P1+P2+P3-P4.
* Formula **214** - P1+P2+P3+P4-P5.
* Formula **215** - P1+P2-P3-P4.
* Formula **216** - IF P2 <= P1 and P1 <= P3 THEN 1 ELSE 0.
* Formula **217** - P1+P2+P3+P4+P5-P6.
* Formula **218** - IF P2 < P1 and P1 <= P3 THEN 1 ELSE 0.
* Formula **219** - IF P2 <= P1 and P1 < P3 THEN 1 ELSE 0.
* Formula **220** - IF P2 < P1 THEN P2 ELSE P1.
* Formula **221** - IF P2 > P1 THEN P2 ELSE P1.
* Formula **222** - P1+P2+P3-P4-P5-P6-P7.
* Formula **223** - P1+P2+P3-P4-P5-P6-P7-P8.
* Formula **224** - P1+P2+P3+P4+P5+P6-P7.
* Formula **225** - Reserved.
* Formula **226** - P1+P2-P3-P4-P5-P6-P7-P8.
* Formula **227** - P1+P2+P3-P4-P5.
* Formula **228** - P1+P2+P3-P4-P5-P6-P7-P8-P9-P10-P11-P12.
* Formula **229** - P1+P2+P3+P4+P5+P6+P7+P8+P9-P10-P11-P12.
* Formula **230** - P1+P2+P3+P4+P5+P6+P7+P8+P9-P10.
* Formula **231** - IF P1 > 0 THEN 0 ELSE P1.
* Formula **232** - P1+P2+P3+P4+P5-P6-P7-P8.
* Formula **233-50000** - Reserved.
* Formula **100000+** - Reserved for custom usage (user space).

## Компіляція

Цей модуль слід компілювати за допомогою компілятора Golang 1.12 або пізнішої версії.

```
go get go.mongodb.org/mongo-driver/bson go.mongodb.org/mongo-driver/mongo go.mongodb.org/mongo-driver/mongo/options
go build
```

Для запуску виконуваного файлу з json-scada-dir/bin/ виконуваного файлу потрібно завантажити файл конфігурації з папки ../conf/.

Організація файлів проекту повинна нагадувати структуру нижче.

```
json-scada-dir/                          # main folder
  bin/calculations                       # executable
  conf/json-scada.json                   # config file
  src/calculations/calculations.go       # source file
```

## Обробити аргументи командного рядка та змінні середовища

Цей драйвер має такі аргументи командного рядка та еквівалентні змінні середовища.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_INSTANCE**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_CALCULATIONS_LOGLEVEL**.
* _**3rd arg. - Period of Calculation**_ [String] - Time period of calculation in seconds. **Optional argument, default=2.0**. Env. variable: **JS_CALCULATIONS_PERIOD**.
* _**4th arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Аргументи командного рядка мають перевагу над змінними середовища.

## Обробка зразка екземпляра

Якщо не знайдено, буде створено запис _processInstance_ зі значеннями за замовчуванням. Він може бути використаний для налаштування деяких параметрів та обмеження вузлів, дозволених для запуску екземплярів.

Див. також 

* [Schema Documentation](../../docs/schema.md) 

