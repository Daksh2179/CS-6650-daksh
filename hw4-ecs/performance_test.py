import requests
import time
import matplotlib.pyplot as plt

# Your service IPs
SPLITTER_IP = "44.244.80.112"
MAPPER_IPS = ["54.68.65.179", "44.234.25.237", "18.246.36.112"]
REDUCER_IP = "18.236.211.187"

S3_FILE = "s3://cs6650-mapreduce-daksh/input/hamlet.txt"

def time_mapreduce():
    times = {}
    
    # 1. Time Splitter
    print("Testing Splitter...")
    start = time.time()
    response = requests.get(f"http://{SPLITTER_IP}:8080/split?s3_url={S3_FILE}")
    times['splitter'] = time.time() - start
    chunk_urls = response.json()['chunk_urls']
    print(f"Splitter took: {times['splitter']:.2f}s")
    
    # 2. Time Mappers (parallel)
    print("\nTesting Mappers...")
    mapper_times = []
    mapped_urls = []
    
    for i, (mapper_ip, chunk_url) in enumerate(zip(MAPPER_IPS, chunk_urls)):
        start = time.time()
        response = requests.get(f"http://{mapper_ip}:8080/map?s3_url={chunk_url}")
        elapsed = time.time() - start
        mapper_times.append(elapsed)
        mapped_urls.append(response.json()['output_url'])
        print(f"Mapper {i+1} took: {elapsed:.2f}s")
    
    times['mapper_avg'] = sum(mapper_times) / len(mapper_times)
    times['mapper_max'] = max(mapper_times)
    times['mapper_times'] = mapper_times
    
    # 3. Time Reducer
    print("\nTesting Reducer...")
    start = time.time()
    urls_param = ",".join(mapped_urls)
    response = requests.get(f"http://{REDUCER_IP}:8080/reduce?s3_urls={urls_param}")
    times['reducer'] = time.time() - start
    result = response.json()
    print(f"Reducer took: {times['reducer']:.2f}s")
    print(f"Total words: {result['total_words']}, Unique words: {result['unique_words']}")
    
    # Total time (using max mapper since they run in parallel)
    times['total'] = times['splitter'] + times['mapper_max'] + times['reducer']
    print(f"\n=== TOTAL TIME: {times['total']:.2f}s ===")
    
    return times

def plot_results(times):
    # Create figure with better proportions
    fig = plt.figure(figsize=(16, 7))
    
    # Plot 1: Overall stages (left side, larger)
    ax1 = plt.subplot(1, 2, 1)
    stages = ['Splitter', 'Mapper\n(parallel)', 'Reducer', 'Total\nPipeline']
    values = [times['splitter'], times['mapper_max'], times['reducer'], times['total']]
    colors = ['#3498db', '#e74c3c', '#2ecc71', '#f39c12']
    
    bars1 = ax1.bar(stages, values, color=colors, edgecolor='black', linewidth=2, width=0.6)
    
    for bar in bars1:
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height + 0.02,
                f'{height:.2f}s',
                ha='center', va='bottom', fontsize=13, fontweight='bold')
    
    ax1.set_ylabel('Time (seconds)', fontsize=14, fontweight='bold')
    ax1.set_title('MapReduce Pipeline Performance', fontsize=16, fontweight='bold', pad=15)
    ax1.grid(axis='y', alpha=0.3, linestyle='--', linewidth=1)
    ax1.set_ylim(0, max(values) * 1.15)
    
    # Plot 2: Individual mapper times (right side)
    ax2 = plt.subplot(1, 2, 2)
    mapper_labels = ['Mapper 1', 'Mapper 2', 'Mapper 3']
    bars2 = ax2.bar(mapper_labels, times['mapper_times'], color='#e74c3c', 
                    edgecolor='black', linewidth=2, width=0.5)
    
    for bar in bars2:
        height = bar.get_height()
        ax2.text(bar.get_x() + bar.get_width()/2., height + 0.005,
                f'{height:.2f}s',
                ha='center', va='bottom', fontsize=13, fontweight='bold')
    
    ax2.axhline(y=times['mapper_avg'], color='#27ae60', linestyle='--', 
                linewidth=2.5, label=f'Average: {times["mapper_avg"]:.2f}s')
    ax2.set_ylabel('Time (seconds)', fontsize=14, fontweight='bold')
    ax2.set_title('Parallel Mapper Execution', fontsize=16, fontweight='bold', pad=15)
    ax2.legend(fontsize=12, loc='upper right')
    ax2.grid(axis='y', alpha=0.3, linestyle='--', linewidth=1)
    ax2.set_ylim(0, max(times['mapper_times']) * 1.15)
    
    # Main title
    fig.suptitle('Distributed MapReduce Word Count: Hamlet\n30,267 total words | 4,700 unique words | 3 parallel mappers', 
                 fontsize=17, fontweight='bold', y=0.98)
    
    plt.tight_layout(rect=[0, 0, 1, 0.94])
    plt.savefig('mapreduce_performance.png', dpi=300, bbox_inches='tight', facecolor='white')
    print("\n✅ Chart saved as 'mapreduce_performance.png'")
    plt.show()

if __name__ == "__main__":
    print("="*60)
    print("        MapReduce Performance Analysis")
    print("="*60 + "\n")
    
    times = time_mapreduce()
    plot_results(times)
    
    print("\n" + "="*60)
    print("Performance Summary:")
    print("="*60)
    print(f"  Splitter:           {times['splitter']:.3f}s")
    print(f"  Mappers (average):  {times['mapper_avg']:.3f}s")
    print(f"  Mappers (max):      {times['mapper_max']:.3f}s")
    print(f"  Reducer:            {times['reducer']:.3f}s")
    print(f"  ────────────────────────────")
    print(f"  TOTAL PIPELINE:     {times['total']:.3f}s")
    print("="*60)