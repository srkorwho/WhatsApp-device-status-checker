# WhatsApp-device-status-checker
A simple tool to measure message delivery timing on WhatsApp by analyzing the delay between server receipt and phone delivery. Check if the phone active/deactive/using whatsapp

## Background

This project is based on research presented in the paper "Careless Whisper" by University of Vienna researchers, which demonstrates how delivery receipt timing can reveal information about a target's phone state. The implementation was inspired by [this YouTube video](https://www.youtube.com/watch?v=HHEQVXNCrW8).

## How It Works

WhatsApp messages go through two stages:
1. Message reaches WhatsApp server (1st tick)
2. Message delivered to recipient's phone (2nd tick)

This tool measures the time between these two events. According to the research:
- < 100ms typically indicates the app is open
- 100-500ms suggests phone is unlocked
- 500-2000ms indicates phone is locked
- \> 2000ms suggests phone is offline or slow connection

## Legal & Ethical Notice

This tool is for **educational and research purposes only**. Only use it on:
- Your own phone numbers
- Numbers where you have explicit permission

Unauthorized surveillance or monitoring of others without consent may be illegal in your jurisdiction. The author is not responsible for misuse.

## Requirements

- Go 1.19+
- WhatsApp account

## Installation
```bash
git clone https://github.com/yourusername/whatsapp-delivery-timing
cd whatsapp-delivery-timing
go mod init whatsapp-timing
go get go.mau.fi/whatsmeow
go get go.mau.fi/whatsmeow/store/sqlstore
go get modernc.org/sqlite
```

## Usage
```bash
go run main.go
```

1. Scan QR code with WhatsApp
2. Enter target phone number
3. Set message interval in seconds
4. Monitor timing results

## Credits

- Research: University of Vienna - "Careless Whisper" paper
- Video explanation: [[YouTube video link]](https://www.youtube.com/watch?v=HHEQVXNCrW8)
- Library: whatsmeow by tulir

## Disclaimer

This is a proof of concept for security research. Use responsibly and ethically.
