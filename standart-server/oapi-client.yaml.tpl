package: {{ .Env.TPL_PACKAGE }}
output: {{ .Env.TPL_OUTPUT }}
generate:
  client: true
  models: true
output-options:
  nullable-type: true