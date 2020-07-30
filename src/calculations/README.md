# {json:scada} calculations.go

The _calculations_ process is responsible for the execution of predefined (compiled) formulas over parcels to update value of calculated points. The default period of calculation cycle is 2 seconds. The full set of calculated point is recalculated at each cycle.

This method of calculation is convenient and efficient. It does not consume change streams and only writes changed values. The drawback of this method is that there is a latency in the order of the cycle period. The period of calculation can be altered if necessary but it should be reasonable to allow for some spare time.

If it is necessary to calculate values with very low latency, it must be created a new custom change stream process that will recalculate values immediately after each parcel change. A custom formula code (a number over 100000) should be provided for this purpose.

A calculated point must be defined in the _realtimeData_ collection.
It must

* Be marked with _origin="calculated"_.
* Have a _formula_ code defined.
* Have its parcels defined (array of point keys, __id_ of the parcels).



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

The available formulas are listed below.

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

## Compilation

This module should be compiled with the Golang compiler 1.12 or later.

```
go get go.mongodb.org/mongo-driver/bson go.mongodb.org/mongo-driver/mongo go.mongodb.org/mongo-driver/mongo/options
go build
```

The executable must be copied or symlinked to run from the json-scada-dir/bin/ to be able to load the config file from the ../conf/ folder.

The organization of the project files should resemble the structure below.

```
json-scada-dir/                          # main folder
  bin/calculations                       # executable
  conf/json-scada.json                   # config file
  src/calculations/calculations.go       # source file
```

