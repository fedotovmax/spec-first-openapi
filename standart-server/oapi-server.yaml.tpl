package: {{ .Env.TPL_PACKAGE }}
output: {{ .Env.TPL_OUTPUT }}
generate:
  chi-server: true
  models: true
output-options:
  nullable-type: true