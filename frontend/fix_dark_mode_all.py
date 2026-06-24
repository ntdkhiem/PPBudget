import re
import os

directories = [
    "app",
    "components"
]

replacements = {
    r'\bbg-white\b(?! dark:bg-)': 'bg-white dark:bg-slate-900',
    r'\bbg-slate-50\b(?! dark:bg-)': 'bg-slate-50 dark:bg-slate-800/50',
    r'\bbg-slate-100\b(?! dark:bg-)': 'bg-slate-100 dark:bg-slate-800',
    r'\btext-slate-900\b(?! dark:text-)': 'text-slate-900 dark:text-slate-100',
    r'\btext-slate-800\b(?! dark:text-)': 'text-slate-800 dark:text-slate-200',
    r'\btext-slate-700\b(?! dark:text-)': 'text-slate-700 dark:text-slate-300',
    r'\btext-slate-600\b(?! dark:text-)': 'text-slate-600 dark:text-slate-400',
    r'\bborder-slate-200\b(?! dark:border-)': 'border-slate-200 dark:border-slate-700',
    r'\bborder-slate-100\b(?! dark:border-)': 'border-slate-100 dark:border-slate-800',
    r'\bborder-slate-200/60\b(?! dark:border-)': 'border-slate-200/60 dark:border-slate-700/60',
    r'\bbg-slate-50/50\b(?! dark:bg-)': 'bg-slate-50/50 dark:bg-slate-800/50',
    r'\bhover:bg-slate-100\b(?! dark:hover:bg-)': 'hover:bg-slate-100 dark:hover:bg-slate-800',
    r'\bhover:bg-slate-50\b(?! dark:hover:bg-)': 'hover:bg-slate-50 dark:hover:bg-slate-800/50',
    r'\bbg-indigo-50\b(?! dark:bg-)': 'bg-indigo-50 dark:bg-indigo-900/30',
    r'\btext-indigo-900\b(?! dark:text-)': 'text-indigo-900 dark:text-indigo-300',
    r'\btext-purple-900\b(?! dark:text-)': 'text-purple-900 dark:text-purple-300',
}

files_modified = 0

for root_dir in directories:
    for dirpath, _, filenames in os.walk(root_dir):
        for file in filenames:
            if file.endswith('.tsx') or file.endswith('.ts'):
                filepath = os.path.join(dirpath, file)
                
                with open(filepath, 'r') as f:
                    content = f.read()
                
                original = content
                for pattern, repl in replacements.items():
                    content = re.sub(pattern, repl, content)
                    
                if original != content:
                    with open(filepath, 'w') as f:
                        f.write(content)
                    files_modified += 1

print(f"Modified {files_modified} files")
