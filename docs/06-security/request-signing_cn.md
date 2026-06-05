# 请求签名

agent message 在签名前会进行 canonicalization。验证时使用已注册的公钥，并拒绝被修改过的 payload。
