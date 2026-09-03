## iris的gorm自用模板
## 说明
- 改config的东西 改go_deploy.yml 里面 配置变量
    - DOCKERHUB_USERNAME
    - DOCKERHUB_TOKEN
    - 自动上传的tag为仓库名称 可以自己改
    - 已配置跨域
- 想要构建 就在提交文本中 deploy: 这样开头就行了
- 已配置i18n 和 csrf 
- 已升级为 golang 1.25
- 已加入了小程序得部分
  - jwtToken
  - sdk
  - routers下面得api.go
  - validator的mini.go
