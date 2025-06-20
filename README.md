# Code Runner 

Миссия проекта **Code Runner**  — предоставить платформу, где студенты могут совершенствовать свои навыки программирования посредством решения различных задач на разных языках программирования.


## Поддержа языков

Платформа поддерживает 1 язык программирования Python.

## Дизайн системы

![Описание изображения](doc/services.png)


## Рабочие Endpoints:

### Frontend Service

- `GET /task/{id}` \- Получить задачу по UUID

### REST API Service

- `POST /run` \-  Отправить код на проверку

&nbsp;
&nbsp;
## Docker CLI:
###  PostgreSQL
`$ docker run --name postgres2 -p 5432:5432 -e POSTGRES_USER=auth -e POSTGRES_PASSWORD=123  -e POSTGRES_DB=auth -d postgres:16`

###  RabbitMQ
`$ docker run -d --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3-management`

