/** Создаёт схему "spyder" в базе данных "tlc"
  */

\set s "spyder"

\c "tlc"

DROP SCHEMA IF EXISTS :s CASCADE;

CREATE SCHEMA IF NOT EXISTS :s;
COMMENT ON SCHEMA :s IS $$`spyders data$$;
/** Таблица сообщений от приложений
  */
CREATE TABLE :s."spy" (
  drec              timestamp ( 0 ) without time zone NOT NULL DEFAULT "now"(),
  app_name          text NOT NULL,
  app_version       text NOT NULL,
  boot_unique_id    text,
  build_cpu_arch    text,
  current_cpu_arch  text,
  kernel_type       text,
  kernel_version    text,
  host_name         text,
  host_unique_id    text,
  product_name      text
);
ALTER TABLE :s."spy" OWNER TO "tlc";
COMMENT ON TABLE :s."spy" IS 'Таблица сообщений от приложений';
COMMENT ON COLUMN :s."spy"."drec" IS 'Время записи';
COMMENT ON COLUMN :s."spy"."app_name" IS 'Имя приложения';
COMMENT ON COLUMN :s."spy"."app_version" IS 'Версия приложения';
COMMENT ON COLUMN :s."spy"."boot_unique_id" IS 'Уникальный ID загрузки хоста';
COMMENT ON COLUMN :s."spy"."build_cpu_arch" IS 'Архтитектура CPU для которой собиралась Qt';
COMMENT ON COLUMN :s."spy"."current_cpu_arch" IS 'Архитектура CPU хоста';
COMMENT ON COLUMN :s."spy"."kernel_type" IS 'Тип ядра ОС';
COMMENT ON COLUMN :s."spy"."kernel_version" IS 'Версия ядра ОС';
COMMENT ON COLUMN :s."spy"."host_name" IS 'Имя хоста';
COMMENT ON COLUMN :s."spy"."host_unique_id" IS 'Уникальный ID хоста';
COMMENT ON COLUMN :s."spy"."product_name" IS 'Название и версия ОС';
/** Таблица команд отправляемых в ответ на сообщение
  */
CREATE TABLE :s."actions" (
  host_unique_id        text NOT NULL,
  action                text NOT NULL
);
ALTER TABLE :s."actions" OWNER TO "tlc";
COMMENT ON TABLE :s."actions" IS 'Команды отправляемые хостам в ответ на сообщение';
COMMENT ON COLUMN :s."actions"."host_unique_id" IS 'Уникальный ID хоста';
COMMENT ON COLUMN :s."actions"."action" IS 'Акция';
/** Таблица для связи spy.host_unique_id с предприятием и местом утановки
  */
CREATE TABLE :s."place" (
  host_unique_id        text UNIQUE NOT NULL,
  company_id            int NOT NULL REFERENCES "tlc"."company"("id"),
  place                 text,
  comment               text
);
ALTER TABLE :s."place" OWNER TO "tlc";
COMMENT ON TABLE :s."place" IS 'Места размещения компьютеров';
/** Пользователь
  */
\set user "spyder"
DROP USER IF EXISTS :user;
CREATE USER :user PASSWORD 'spyder';
GRANT USAGE ON SCHEMA :s TO :user;
GRANT ALL ON ALL TABLES IN SCHEMA :s TO :user;
