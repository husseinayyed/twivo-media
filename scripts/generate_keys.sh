#!/bin/bash
set -e

echo "🔑 Generating Ed25519 keys for twivo-media..."

if ! command -v openssl &> /dev/null; then
    echo "❌ OpenSSL is not installed. Please install it first:"
    echo "   sudo apt install openssl    # Ubuntu/Debian"
    echo "   brew install openssl        # macOS"
    exit 1
fi

mkdir -p keys

if [ -f "keys/private.pem" ] || [ -f "keys/public.pem" ]; then
    echo "⚠️  Keys already exist in keys/"
    read -p "Do you want to overwrite them? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ Aborted."
        exit 0
    fi
fi

openssl genpkey -algorithm Ed25519 -out keys/private.pem
openssl pkey -in keys/private.pem -pubout -out keys/public.pem
chmod 600 keys/private.pem
chmod 644 keys/public.pem

if ! grep -q "^keys/" .gitignore 2>/dev/null; then
    echo -e "\n# Ed25519 keys\nkeys/" >> .gitignore
    echo "✅ Added keys/ to .gitignore"
fi

echo ""
echo "✅ Keys generated successfully:"
echo "   - Private key: $(pwd)/keys/private.pem (🔒 KEEP SECRET!)"
echo "   - Public key:  $(pwd)/keys/public.pem (📢 Safe to share)"
echo ""
echo "🔐 Security reminder:"
echo "   - NEVER commit private.pem (GitHub will ignore keys/ via .gitignore)"
echo "   - public.pem is REQUIRED for the API and safe to commit"