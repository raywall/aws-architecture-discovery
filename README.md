# AWS Architecture Discovery

Aplicação Go com arquitetura hexagonal para unir inventário AWS e dependências APM em um grafo e gerar Mermaid, CSV para draw.io ou JSON orientado a C4. Um único executável atende CLI, tarefa ECS e Lambda. O código não consulta valores de parâmetros SSM, mensagens ou registros de tabelas.

## Executar localmente

Requer Go 1.27 (versão registrada no `go.mod`) e credenciais AWS válidas para os modos `aws` e `hybrid`.

```sh
make build
cp config.example.yaml config.yaml
# Ajuste AWS_REGION e, opcionalmente, AWS_PROFILE no arquivo.
make run
# Ou: ./bin/discovery -config config.yaml
```

O arquivo resultante será `./diagrams/architecture.mermaid`. `OUTPUT_DESTINATION` é um **diretório ou prefixo S3**, não um nome de arquivo. A execução seguinte substitui o artefato do mesmo formato. Escritas locais usam arquivo temporário privado e rename atômico. Para histórico no S3, habilite versionamento no bucket.

```sh
DISCOVERY_MODE=hybrid OUTPUT_FORMAT=drawio-csv \
DD_ENV_TARGET=production ./bin/discovery -config config.yaml
```

Selecione a fonte no `config.yaml`:

```yaml
DISCOVERY_MODE: HYBRID # AWS, DATADOG ou HYBRID
```

Ou sobrescreva pelo ambiente: `DISCOVERY_MODE=DATADOG make run`. `AWS` coleta apenas infraestrutura, `DATADOG` coleta apenas dependências APM e `HYBRID` combina as duas fontes.

Forneça `DD_API_KEY` e `DD_APP_KEY` pelo ambiente ou injeção de segredos. Não coloque chaves em arquivos versionados. Para usar somente Datadog, configure `DISCOVERY_MODE=datadog`; credenciais AWS só serão necessárias se o destino for S3.

## Configuração

Variáveis de ambiente prevalecem sobre YAML, que prevalece sobre os padrões. `make run` carrega `config.yaml` por padrão; use `make run CONFIG=outro.yaml` para outro arquivo. Na execução direta do binário, indique `-config config.yaml`. Erro de leitura encerra a execução. Para configuração apenas por variáveis de ambiente, use `make run CONFIG=`.

| Variável | Padrão / finalidade |
|---|---|
| `DISCOVERY_MODE` | `AWS`, `DATADOG` ou `HYBRID`, sem distinção entre maiúsculas e minúsculas; padrão `AWS` |
| `OUTPUT_FORMAT` | `mermaid`, `drawio-csv`, `c4-json`; padrão `mermaid` |
| `OUTPUT_DESTINATION` | `./diagrams/` ou `s3://bucket/prefix/` |
| `AWS_REGION` | Obrigatória para AWS e escrita S3 |
| `AWS_PROFILE` | Perfil do provedor padrão, incluindo SSO na CLI |
| `TARGET_ACCOUNT_ROLE_ARN` | AssumeRole opcional para o inventário |
| `DD_API_KEY`, `DD_APP_KEY` | Credenciais da API Datadog |
| `DD_SITE` | `datadoghq.com`; aceita os sites públicos e governamentais conhecidos |
| `DD_ENV_TARGET` | Ambiente obrigatório para coleta Datadog |
| `DISCOVERY_TIMEOUT` | `10m`, limite total por execução |
| `DD_TRACE_ENABLED` | `false` na CLI, `true` em Lambda/ECS |

O provedor padrão do SDK obtém credenciais de ambiente, arquivos, SSO ou execution/task role. O código não solicita perfis explicitamente em cloud; evite configurar `AWS_PROFILE` em Lambda/ECS. A role opcional da conta alvo é usada **somente para discovery**. A escrita S3 utiliza a identidade original de execução, permitindo guardar o resultado na conta de automação.

## Pipeline e organização

```text
cmd/discovery                  composição das dependências e detecção do runtime
internal/config                Viper e validação
internal/application           caso de uso e portas Provider, Renderer, Sink
internal/domain                grafo em memória e resolução de identidades
internal/adapter/aws           AWS SDK v2, inventário e relações estáticas
internal/adapter/datadog       API pública de dependências APM
internal/adapter/render        Mermaid, draw.io CSV e C4 JSON
internal/adapter/storage       arquivos locais e S3
internal/adapter/telemetry     instrumentação nativa dd-trace-go
```

O motor de grafos é implementado no domínio com mapas de nós e arestas, sem dependência de SysGraph: o requisito permite um motor equivalente. O domínio não importa SDKs, Viper ou Datadog. Coletores e renderers são selecionados no composition root.

A identidade estrutural usa ARN; tópicos Kafka usam cluster + nome. Um serviço APM é mesclado apenas quando `service`/`env` nas tags AWS ou `DD_SERVICE`/`DD_ENV` na configuração identificam um único recurso. Colisões e ambientes diferentes permanecem separados. Nomes parecidos não são evidência suficiente. O filtro `DD_ENV_TARGET` se aplica à fonte Datadog; o inventário AWS continua abrangendo a região da conta selecionada.

Relações estáticas incluem Lambda Event Source Mappings, SNS subscriptions, integrações REST/HTTP/WebSocket do API Gateway, fontes Lambda/DynamoDB do AppSync e referências explícitas a ARNs/URLs em configurações Lambda/ECS. Variáveis e segredos de configuração não são exportados. Referências apenas por nomes curtos ou por expressões dinâmicas não são inferidas.

Os nós e arestas são ordenados e deduplicados. Rótulos visuais são escapados. A mesma topologia gera o mesmo conteúdo, sem timestamps variáveis. Uma falha de coleta impede a publicação de um desenho parcial. O SDK faz até três tentativas; Datadog repete 429/5xx até três vezes, com espera crescente e cancelamento. O timeout HTTP é 30 segundos e o contexto limita toda a execução.

## Cobertura e limites das fontes

| Fonte | Cobertura |
|---|---|
| Tags AWS | Recursos retornados por Resource Groups Tagging API; complementam as listagens específicas |
| Lambda | Funções, configuração, event source mappings |
| ECS | Clusters, serviços, tarefas em execução e container instances; referências de task definitions |
| API Gateway | APIs REST e v2, integrações e referências a recursos inventariados |
| AppSync | GraphQL APIs e referências a Lambda/DynamoDB em data sources |
| SQS / SNS | Filas, tópicos e subscriptions com destino ARN |
| MSK | Clusters provisionados/serverless; tópicos em provisionados compatíveis |
| DynamoDB / SSM | Tabelas e metadados de parâmetros, sem conteúdo |
| Datadog | Serviços e arestas `calls` retornados pelo endpoint de dependências |

A [API Datadog de dependências](https://docs.datadoghq.com/api/latest/service-dependencies/get-all-apm-service-dependencies/) está em beta e retorna relações por serviço, com janela padrão de uma hora. Ela não fornece um inventário garantido de operações RPC, tabelas, publishers/subscribers ou TPS. Esses componentes só aparecem quando representados como serviços pela instrumentação APM. O projeto não inventa detalhes ausentes e não implementa o enriquecimento opcional de métricas. Use as permissões APM restritas indicadas para sua organização (`apm_dependencies_read` e, quando aplicável, `apm_service_map_read`); um 403 é apresentado como erro, sem tentar escopos mais amplos.

A [API MSK ListTopics](https://docs.aws.amazon.com/msk/latest/developerguide/msk-list-topics.html) é limitada a clusters provisionados compatíveis (Kafka 3.6+). Serverless e versões anteriores entram como clusters e produzem um aviso de ausência do inventário de tópicos; tópicos evidenciados por mappings Lambda ainda podem aparecer. Não há conexão direta a brokers Kafka externos. Recursos sem atividade APM não ganham arestas de runtime automaticamente.

O modo `c4-json` usa o schema versionado `aws-architecture-discovery/c4-v1`: elementos `container` com tecnologia AWS e relações com origem da evidência. Não é o formato de workspace Structurizr nem uma classificação automática de sistemas/pessoas. O CSV do draw.io preserva conectividade, mas não os rótulos individuais das arestas; Mermaid e JSON preservam tipo e fonte. Importe CSV por **Arrange → Insert → Advanced → CSV**, incluindo as linhas de configuração.

## AWS: permissões e implantação

- `deployments/discovery-read-policy.json`: permissões de metadados realmente usadas. Inclui ECS, AppSync e tópicos MSK, que não estavam na matriz inicial. A região é controlada na configuração; ações de listagem usam `Resource: "*"`.
- `deployments/output-policy.example.json`: exemplo de `s3:PutObject` restrito ao prefixo; substitua o nome do bucket. Bucket com SSE-KMS pode exigir permissões adicionais na chave.
- AssumeRole exige `sts:AssumeRole` restrito ao ARN da role alvo e trust policy correspondente. A role alvo recebe a política de leitura; a identidade original recebe a política de saída.
- `sts:GetCallerIdentity` é usado para obter conta/partição e não exige uma concessão IAM explícita.

```sh
make lambda
sam validate --lint --template-file deployments/template.yaml
sam deploy --guided --template-file deployments/template.yaml
```

O template SAM usa bucket existente, Lambda ARM64, timeout de 15 minutos, concorrência reservada 1, EventBridge diário e `POST /discover` protegido por AWS IAM. A implantação é manual; nenhum recurso foi criado automaticamente. SAM adiciona permissões de logging e invocação aos recursos gerados. Para execuções longas, o timeout síncrono do API Gateway pode expirar antes do discovery: prefira o agendamento ou invocação Lambda assíncrona. O artefato é escrito no S3, não retornado no corpo da API.

Para ECS, construa a imagem do `Dockerfile`, execute como tarefa com task role de leitura e destino S3, e use EventBridge Scheduler com alvo `RunTask` quando precisar de agendamento. O processo executa uma varredura e termina; não mantém um servidor HTTP. Dimensionamento inicial de 512 MiB na Lambda é apenas um ponto de partida: valide memória, duração e custo na conta real.

## Observabilidade

O motor usa `dd-trace-go/v2` diretamente, com spans de execução e de coleta, contagens de nós/arestas, erros e logs JSON via `slog`. Não usa uma ponte OpenTelemetry. Os identificadores de trace/span são incluídos no log de sucesso. Há flush ao final de cada invocação Lambda e Stop na CLI/ECS.

A [biblioteca Go do Datadog](https://docs.datadoghq.com/tracing/trace_collection/automatic_instrumentation/dd_libraries/go/) precisa de um destino de ingestão de traces: configure Agent em ECS/CLI ou a extensão Datadog na Lambda. A instrumentação no código não substitui o transporte. O template permite informar `DatadogExtensionLayerArn`; injete a chave/secret e demais configurações de ingestão conforme seu ambiente. Sem Agent/extensão configurado, a geração do desenho funciona, mas a entrega dos traces não está garantida. As credenciais para ler a API APM são independentes desse transporte.

## Verificação

```sh
make test
make vet
make build
make lambda
```

Testes sem credenciais verificam identidade/ambiguidade, ordem determinística, falhas e cancelamento do pipeline, contrato HTTP Datadog, paginação Lambda/SQS com SDK real contra servidor simulado, referências de integração, formatos e escrita local/S3 simulada. Não substituem uma execução integrada contra AWS/Datadog. A cobertura por pacote é exibida por `make test`; domínio e aplicação têm cobertura acima de 80%, e os coletores têm cobertura parcial.
# aws-architecture-discovery
