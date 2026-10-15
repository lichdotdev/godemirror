echo "nested $(echo "inner $(date) more" 'single') done"
echo "brace ${var:-"default value"} after"
echo 'single $not ${expanded}' "double \"escaped\" \$dollar"
echo "unterminated
continues on next line $x
ends here"
echo `backtick $(sub) "q"` and `unterminated
next`
echo $(( 1 + (2 * 3) )) $((x)) $( (sub) ) ${a{b}c}
echo "a$"b "c$" "$" '$'
echo "${a}${b}" "$a$b" "$1$2"
echo ${x/"pat"/'rep'} "${y#'*'}"
echo $'ansi\n' $"locale"
echo "dollar at end $
echo \"escaped\" \$x \\ \
echo -n --long-opt -x1 --a-b=c a=b 3 3a a3 1-2
echo "mixed 'quotes' inside" 'mixed "quotes" inside'
x+=1; y=2 z=3
	#!not shebang
 # comment after space
echo 🎉 "🎉 $🎉" 日本語=1 Ωmega
