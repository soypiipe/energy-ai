// La especificación se pide con ruta relativa: funciona igual en /docs/ (API directa) y en /api/docs/ (detrás de nginx).
window.ui = SwaggerUIBundle({
  url: '../openapi.yaml',
  dom_id: '#swagger-ui',
  deepLinking: true,
  docExpansion: 'list',
  persistAuthorization: false,
  tryItOutEnabled: true,
})
