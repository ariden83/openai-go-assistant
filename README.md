# openai-go-assistant

**OpenAI Go Assistant** is a Go project that allows you to interact with the OpenAI API to automate various code generation and optimization operations. Designed for developers, this wizard makes creating Go code easy by offering advanced features, such as generating code from natural instructions, optimizing existing code, fixing errors, and adding unit tests.

## Features

- **Code generation**: Create Go code from simple natural language instructions by leveraging the power of the OpenAI API. The wizard generates initial code for functions, structures, algorithms, and more.

- **Error Correction**: Analyze code to automatically detect and correct syntax, logic, or optimization errors, making the debugging process faster and more efficient.

- **Code Optimization**: Rewrite and optimize existing code to improve performance, reduce complexity, or adhere to Go programming best practices.

- **Unit test generation**: Generate Go unit tests associated with code to ensure feature coverage and automatically validate expected behavior.

## Requirements

Install goimport tools pour réparer les imports manquant

> go install golang.org/x/tools/cmd/goimports@latest

Install staticCheck pour détecter les fonctions non utilisées dans le code généré.

> go install honnef.co/go/tools/cmd/staticcheck@latest

## Installation

1. Clone the repository :

```shell
git clone https://github.com/ariden83/openai-go-assistant.git
cd openai-go-assistant
```

2. Configure your OpenAI API keys:

Create an **.env** file in the root folder and add your OpenAI API key:

```env
OPENAI_API_KEY=your_openai_api_key
```

## Use

Once installed and configured, you can run commands to use the different features of the wizard.

```
go run ./...
```

## Contribution

Contributions are welcome! If you want to improve OpenAI Go Assistant, feel free to open an issue or submit a pull request.

## Licence

This project is under the MIT license.
