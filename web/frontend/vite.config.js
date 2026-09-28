import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Endereco do servidor Go (backend). Altere aqui se o backend noutra porta.
const API_BACKEND = process.env.VIFIC_API ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],

  // O servidor Go serve a aplicacao compilada a partir da raiz (/),
  // por isso os caminhos dos ficheiros nao podem ter prefixo.
  base: '/',

  server: {
    host: true,
    port: 5173,
    allowedHosts: ['.monkeycode-ai.live'],
    // Em desenvolvimento, o frontend fala com a API pelo mesmo origin,
    // evitando configuracao de CORS e problemas de cookies.
    proxy: {
      '/api': {
        target: API_BACKEND,
        changeOrigin: true,
      },
    },
  },

  preview: {
    host: true,
    port: 5173,
    allowedHosts: ['.monkeycode-ai.live'],
    proxy: {
      '/api': {
        target: API_BACKEND,
        changeOrigin: true,
      },
    },
  },

  build: {
    outDir: 'dist',
    sourcemap: true,
    // Separar vendor evita que uma alteracao no codigo invalide tudo.
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom'],
        },
      },
    },
  },
})

