module.exports = {
  devServer: {
    proxy: {
      '^/Invoke/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true
      },
    }
  },
  transpileDependencies: [
    "vuetify"
  ],
  publicPath: ''
}