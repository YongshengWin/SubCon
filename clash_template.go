package main

const clashTemplateYAML = `port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
ipv6: true

dns:
  enable: true
  listen: 0.0.0.0:53
  enhanced-mode: fake-ip
  nameserver:
    - 223.5.5.5
    - 119.29.29.29

rule-providers:
  google:
    type: http
    behavior: classical  
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Google/Google.yaml" 
    path: ./ruleset/google.yaml
    interval: 86400
  youtube:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/YouTube/YouTube.yaml"
    path: ./ruleset/youtube.yaml
    interval: 86400
  netflix:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Netflix/Netflix.yaml"
    path: ./ruleset/netflix.yaml
    interval: 86400
  spotify:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Spotify/Spotify.yaml"
    path: ./ruleset/spotify.yaml
    interval: 86400
  telegram:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Telegram/Telegram.yaml"
    path: ./ruleset/telegram.yaml
    interval: 86400
  chatgpt:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml"
    path: ./ruleset/chatgpt.yaml
    interval: 86400
  claude:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Claude/Claude.yaml"
    path: ./ruleset/claude.yaml
    interval: 86400
  one-one-five:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/115/115.yaml"
    path: ./ruleset/one-one-five.yaml
    interval: 86400
  bilibili:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Bilibili/Bilibili.yaml"
    path: ./ruleset/bilibili.yaml
    interval: 86400
  microsoft:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Microsoft/Microsoft.yaml"
    path: ./ruleset/microsoft.yaml
    interval: 86400
  instagram:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Instagram/Instagram.yaml"
    path: ./ruleset/instagram.yaml
    interval: 86400
  twitter:
    type: http
    behavior: classical
    url: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Twitter/Twitter.yaml"
    path: ./ruleset/twitter.yaml
    interval: 86400

proxies: []
# 说明：此处的数组留空，由订阅后端服务动态插入具体的节点配置字典 (Dictionary)

proxy-groups:
  - { name: '节点选择', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Global.png', proxies: ['自动选择', 'DIRECT'] }
  - { name: '自动选择', type: url-test, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Urltest.png', proxies: [], url: 'http://www.gstatic.com/generate_204', interval: 300, tolerance: 1}
  - { name: 'Google', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Google.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'YouTube', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/YouTube.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'ChatGPT', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/OpenAI.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'Claude', type: select, icon: 'https://raw.githubusercontent.com/Sereinfy/Clash/main/icons/anthropic.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'Instagram', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Instagram.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'X', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Twitter.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'Netflix', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Netflix.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'Spotify', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Spotify.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: 'Telegram', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Telegram.png', proxies: ['节点选择', '自动选择', 'DIRECT'] }
  - { name: '微软服务', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/master/Color/Microsoft.png', proxies: ['DIRECT', '节点选择', '自动选择'] }
  - { name: '哔哩哔哩', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/refs/heads/master/Color/Bili.png', proxies: ['DIRECT', '节点选择', '自动选择'] }
  - { name: '115', type: select, icon: 'https://raw.githubusercontent.com/Orz-3/mini/refs/heads/master/Color/115.png', proxies: ['DIRECT', '节点选择', '自动选择'] }
# 说明：生成服务需要将提取出的“节点名称列表”追加（Append）到上述每一个策略组的 proxies 数组中。

rules:
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve 
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
  - PROCESS-NAME,com.anthropic.claude,Claude
  - DOMAIN-SUFFIX,local,DIRECT
  - DOMAIN-SUFFIX,claudeusercontent.com,Claude
  - DOMAIN-SUFFIX,claude.com,Claude
  - DOMAIN-KEYWORD,claude,Claude

  - RULE-SET,google,Google
  - RULE-SET,youtube,YouTube
  - RULE-SET,chatgpt,ChatGPT
  - RULE-SET,claude,Claude
  - RULE-SET,instagram,Instagram
  - RULE-SET,twitter,X
  - RULE-SET,netflix,Netflix
  - RULE-SET,spotify,Spotify
  - RULE-SET,telegram,Telegram
  - RULE-SET,bilibili,哔哩哔哩
  - RULE-SET,microsoft,微软服务
  - RULE-SET,one-one-five,115
  - MATCH,节点选择
`
