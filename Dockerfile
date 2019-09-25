FROM ubuntu:18.04
# docker run --name db -it -p 5432:5432 -p 5000:5000 tech-db-server:latest /bin/bash
# docker run --name db -it -p 5432:5432 tech-db-server:latest /bin/bash
# docker start db
MAINTAINER smet_k

ENV TZ=Europe/Moscow
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Обвновление списка пакетов
RUN apt-get -y update
RUN apt install -y git wget gcc gnupg
RUN apt install openssl

#
# Установка postgresql
#
ENV PGVER 11

RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ bionic-pgdg main" > /etc/apt/sources.list.d/pgdg.list

# get the signing key and import it
RUN wget https://www.postgresql.org/media/keys/ACCC4CF8.asc
RUN apt-key add ACCC4CF8.asc

# fetch the metadata from the new repo
RUN apt-get update

RUN apt-get install -y  postgresql-$PGVER

# Установка golang
RUN wget https://dl.google.com/go/go1.12.linux-amd64.tar.gz
RUN tar -xvf go1.12.linux-amd64.tar.gz
RUN mv go /usr/local

# Выставляем переменную окружения для сборки проекта
ENV GOROOT /usr/local/go
ENV GOPATH $HOME/go
ENV PATH $GOPATH/bin:$GOROOT/bin:$PATH

# Копируем исходный код в Docker-контейнер
WORKDIR /server
COPY . .

# Объявлем порт сервера
EXPOSE 5000

# Run the rest of the commands as the ``postgres`` user created by the ``postgres-$PGVER`` package when it was ``apt-get installed``
USER postgres

# Create a PostgreSQL role named ``docker`` with ``docker`` as the password and
# then create a database `docker` owned by the ``docker`` role.
RUN /etc/init.d/postgresql start &&\
    psql --command "CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" &&\
    createdb -O docker docker &&\
    psql docker -f /server/init.sql &&\
    /etc/init.d/postgresql stop

# Adjust PostgreSQL configuration so that remote connections to the
# database are possible.
RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/$PGVER/main/pg_hba.conf

# And add ``listen_addresses`` to ``/etc/postgresql/$PGVER/main/postgresql.conf``
RUN echo "listen_addresses='*'" >> /etc/postgresql/$PGVER/main/postgresql.conf

# Expose the PostgreSQL port
EXPOSE 5432

# Add VOLUMEs to allow backup of config, logs and databases
VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

# Back to the root user
USER root
# Запускаем PostgreSQL и сервер

#RUN ./pemgen.sh

RUN ls

RUN go build -mod=vendor /server/cmd/proxy/main.go
CMD service postgresql start && ./main