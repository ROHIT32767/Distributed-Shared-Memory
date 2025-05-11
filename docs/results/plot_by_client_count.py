import pandas as pd
import matplotlib.pyplot as plt
import os

# Load the CSV file
df = pd.read_csv('average_metrics_by_client_count.csv')

# Ensure the output directory exists
os.makedirs("plots_by_client_count", exist_ok=True)

# Use client_count as x-axis
x = df['client_count']

# Plot each metric (excluding client_count) vs client_count
for column in df.columns:
    if column == 'client_count':
        continue
    plt.figure(figsize=(8, 5))
    plt.plot(x, df[column], marker='o')
    plt.title(f'{column} vs Client Count')
    plt.xlabel('Client Count')
    plt.ylabel(column)
    plt.grid(True)
    plt.tight_layout()
    plt.savefig(f'plots_by_client_count/{column}_vs_client_count.png')
    plt.close()

print("All plots saved in 'plots_by_client_count' folder.")
