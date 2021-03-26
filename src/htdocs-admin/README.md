# admin-manager

## Project setup
```
npm install
```

### Compiles and hot-reloads for development
```
npm run serve
```

### Compiles and minifies for production
```
npm run build
```

### Lints and fixes files
```
npm run lint
```

### Customize configuration
See [Configuration Reference](https://cli.vuejs.org/config/).

### Create new translations automatically

https://www.npmjs.com/package/attranslate

attranslate --srcFile=src/locales/en.json --srcLng=en --srcFormat=nested-json --targetFile=src/locales/es.json --targetLng=es --targetFormat=nested-json --service=google-translate --serviceConfig=c:\temp\gcp-translateapikeys.json