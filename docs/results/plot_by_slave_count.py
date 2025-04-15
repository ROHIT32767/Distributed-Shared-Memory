import pandas as pd
import matplotlib.pyplot as plt
import os

# Load the CSV file
df = pd.read_csv('average_metrics_by_slave_count.csv')

# Ensure the output directory exists
os.makedirs("plots_by_slave_count", exist_ok=True)
x = df['slave_count']
for column in df.columns:
    if column == 'slave_count':
        continue
    plt.figure(figsize=(8, 5))
    plt.plot(x, df[column], marker='o')
    plt.title(f'{column} vs Slave Count')
    plt.xlabel('Slave Count')
    plt.ylabel(column)
    plt.grid(True)
    plt.tight_layout()
    plt.savefig(f'plots_by_slave_count/{column}_vs_slave_count.png')
    plt.close()

print("All plots saved in 'plots_by_slave_count' folder.")
