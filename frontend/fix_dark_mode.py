import re

files = [
    "app/(dashboard)/budgets/page.tsx",
    "app/(dashboard)/settings/rules/page.tsx"
]

replacements = {
    r'\bbg-white\b(?! dark:bg-)': 'bg-white dark:bg-slate-900',
    r'\bbg-slate-50\b(?! dark:bg-)': 'bg-slate-50 dark:bg-slate-800/50',
    r'\bbg-slate-100\b(?! dark:bg-)': 'bg-slate-100 dark:bg-slate-800',
    r'\btext-slate-900\b(?! dark:text-)': 'text-slate-900 dark:text-slate-100',
    r'\btext-slate-800\b(?! dark:text-)': 'text-slate-800 dark:text-slate-100',
    r'\btext-slate-700\b(?! dark:text-)': 'text-slate-700 dark:text-slate-300',
    r'\btext-slate-600\b(?! dark:text-)': 'text-slate-600 dark:text-slate-400',
    r'\bborder-slate-200\b(?! dark:border-)': 'border-slate-200 dark:border-slate-800',
    r'\bborder-slate-100\b(?! dark:border-)': 'border-slate-100 dark:border-slate-800/50',
    r'\bborder-slate-200/60\b(?! dark:border-)': 'border-slate-200/60 dark:border-slate-800/60',
    r'\bbg-slate-50/50\b(?! dark:bg-)': 'bg-slate-50/50 dark:bg-slate-800/50',
    r'\bhover:bg-slate-100\b(?! dark:hover:bg-)': 'hover:bg-slate-100 dark:hover:bg-slate-800',
    r'\bbg-indigo-50\b(?! dark:bg-)': 'bg-indigo-50 dark:bg-indigo-900/30',
    r'\btext-indigo-900\b(?! dark:text-)': 'text-indigo-900 dark:text-indigo-300',
    r'\btext-purple-900\b(?! dark:text-)': 'text-purple-900 dark:text-purple-300',
}

for file in files:
    with open(file, 'r') as f:
        content = f.read()
    
    for pattern, repl in replacements.items():
        content = re.sub(pattern, repl, content)
        
    with open(file, 'w') as f:
        f.write(content)

print("Done")
