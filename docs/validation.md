# Validação local — 2026-10-29

Executado com Go 1.27.0 em macOS ARM64:

- `go test -race -coverprofile=coverage.out ./...`: aprovado.
- `go vet ./...`: aprovado.
- `make build`: executável local gerado em `bin/discovery`.
- `make lambda`: executável Linux ARM64 gerado em `build/bootstrap`.
- `sam validate --lint --template-file deployments/template.yaml`: template aprovado.

Cobertura de statements: domínio 94,9%; aplicação 100%; configuração 91,9%; renderers 98,1%; Datadog 76,5%; storage 69,2%; AWS 21%. Composition root e decorador de tracing não têm testes dedicados. A cobertura AWS concentra-se em paginação Lambda/SQS, falhas de autorização e referências; os demais serviços foram compilados contra o SDK, mas não têm simulação individual completa.

Nenhuma varredura de conta real, chamada autenticada ao Datadog ou implantação cloud foi realizada. O template e a imagem não foram implantados. A exportação draw.io foi validada como CSV, sem importação visual no editor. Os detalhes e limites das APIs estão no README.
