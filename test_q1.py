import sys
input_str = sys.stdin.read().strip()
if input_str:
    arr = list(map(int, input_str.split(',')))
    arr.sort()
    print(','.join(map(str, arr)))
