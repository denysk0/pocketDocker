cat > harness.sh << 'EOF'
#!/bin/sh -eux
cat > /tmp/main.py << 'PY'
n = int(input())
print(n * n)
PY

tests="3:9 5:25 10:100"
for pair in $tests; do
  IFS=: read input want <<< "$pair"
  got=$(printf "%s\n" "$input" | python3 /tmp/main.py)
  if [ "$got" != "$want" ]; then
    echo "FAIL: input=$input expected=$want got=$got"
    exit 1
  else
    echo "OK: $input → $got"
  fi
done

echo ALL TESTS PASSED
EOF
chmod +x harness.sh