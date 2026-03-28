import requests
import os

os.makedirs('models', exist_ok=True)

# Ссылки на модели buffalo_l
urls = [
    ('https://raw.githubusercontent.com/DeepInsight-ML/insightface_model_zoo/main/buffalo_l/det_10g.onnx', 'models/det_10g.onnx'),
    ('https://raw.githubusercontent.com/DeepInsight-ML/insightface_model_zoo/main/buffalo_l/w600k_r50.onnx', 'models/w600k_r50.onnx'),
]

for url, dest in urls:
    if os.path.exists(dest):
        size = os.path.getsize(dest)
        print(f'{dest} exists ({size//1024//1024} MB), skipping')
        continue
    
    print(f'Downloading {dest} from {url}...')
    try:
        r = requests.get(url, stream=True, timeout=120)
        r.raise_for_status()
        total = int(r.headers.get('content-length', 0))
        
        downloaded = 0
        with open(dest, 'wb') as f:
            for chunk in r.iter_content(chunk_size=8192):
                f.write(chunk)
                downloaded += len(chunk)
        
        size = os.path.getsize(dest)
        print(f'Downloaded {dest}: {size} bytes ({size//1024//1024} MB)')
        
        # Проверка размера
        if size < 1024 * 1024:  # < 1MB
            print(f'  WARNING: File too small, may be corrupted')
            os.remove(dest)
    except Exception as e:
        print(f'Error downloading {dest}: {e}')

print('\nDone!')
