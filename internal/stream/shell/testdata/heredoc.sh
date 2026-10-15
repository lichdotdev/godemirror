cat <<EOF rest of line
line one $HOME
  EOF
EOF
cat <<-'END'
	quoted $nothing
END
cat << "Q"
q
Q
cat <<'mixed"
inside
mixed
cat <<<"here string" <<< word
cat < input.txt > output.txt
cat <<
not ended by blank lines

cat <<''
x
