

## Requirements

Install goimport tools pour réparer les imports manquant

> go install golang.org/x/tools/cmd/goimports@latest

Install staticCheck pour détecter les fonctions non utilisées dans le code généré.

> go install honnef.co/go/tools/cmd/staticcheck@latest

Create an **.env** file in the project root to specify the token to use to call the OpenAI API : 
```
OPENAI_API_KEY=...
```
