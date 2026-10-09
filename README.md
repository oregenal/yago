# yago
Simple console utility for [Yandex Disk](https://disk.yandex.ru).

## Build
```console
go build
```
## Startup
First of all you need [OAuth token](https://yandex.ru/dev/id/doc/ru/register-api).  
Required rights: Read all Disk, Write to Disk, Access to Disk info.  
To enter that token use `token` key.
```console
yago token <OAuth_token>
```

## Usage
Ussage information can be called by `help` key.
```console
yago help
```

Call **yago** without any keys show Disk space information.
```console
yago
```
To list directory use `ls` key.  
Show root directory list If no arguments provided.  
Disk path can be prefixed by `disk:/` or `disk:` or `/` or without any prefix.  
```console
yago ls
yago ls <path/dirname>
```

To download file use `down` key. Download directory not implemented.
```console
yago down <path/filename>
```

To upload file use `up` key. Upload directory not implemented.
```console
yago up <locale/path/file> <disk/path/file>
```
To create directory use `mkdir` key.
```console
yago mkdir <path/dirname>
```

To remove directory or file use `rm` key.
```console
yago rm <path/dir|file>
```
