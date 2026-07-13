module.exports = {
  content: ['./vueapp/**/*.vue', './vueapp/**/*.ts'],
  theme: {
    extend: {}
  },
  plugins: [require('daisyui')],
  daisyui: {
    themes: ['night', 'corporate']
  }
}
