import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'

// Tema oscuro de Voltix sobre Aura. Todo componente de PrimeVue (tablas, botones, tarjetas, inputs)
// hereda estos tokens: no se pinta nada a mano.
//  - primary: escala del turquesa de marca #2DD4A7 (el 400 es el color base en modo oscuro).
//  - surface: fondo #0B0F19, tarjetas #131826, elevado/hover #1B2233, bordes #232B3D.
export const VoltixPreset = definePreset(Aura, {
  semantic: {
    primary: {
      50: '#ecfdf8',
      100: '#d1faee',
      200: '#a7f3de',
      300: '#6ee7c8',
      400: '#2DD4A7',
      500: '#14b88e',
      600: '#0d9474',
      700: '#0f755d',
      800: '#115e4d',
      900: '#134e40',
      950: '#042f27',
    },
    colorScheme: {
      dark: {
        surface: {
          0: '#ffffff',
          50: '#F5F7FA',
          100: '#dfe3ea',
          200: '#c5cad6',
          300: '#a4acbc',
          400: '#8A93A6',
          500: '#5f6a80',
          600: '#3a4560',
          700: '#232B3D',
          800: '#1B2233',
          900: '#131826',
          950: '#0B0F19',
        },
      },
    },
  },
})
