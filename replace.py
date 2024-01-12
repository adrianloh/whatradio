import re

# Define the input and output file names
input_file_name = 'main.go'
output_file_name = 'main2.go'

# Regular expression pattern to find and group the part to be reused
pattern = r'log\.Printf\("(.+?)", (.+?)\)'
replacement = r'fmt.Printf("\1\\n", \2)'

# Read the input file and write the modifications to the output file
with open(input_file_name, 'r') as input_file, open(output_file_name, 'w') as output_file:
    for line in input_file:
        # Replace the pattern in each line if it matches
        new_line = re.sub(pattern, replacement, line)
        output_file.write(new_line)

print(f'File processed. Output saved to {output_file_name}.')