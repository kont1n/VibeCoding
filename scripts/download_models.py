#!/usr/bin/env python3
"""Download InsightFace models for VibeCoding project."""

import os
import sys
import urllib.request
import urllib.error

PROJECT_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MODELS_DIR = os.path.join(PROJECT_DIR, 'models')

MODELS = {
    'det_10g.onnx': {
        'url': 'https://huggingface.co/spaces/lea-ondr/insightface-demo/resolve/main/models/det_10g.onnx',
        'size_mb': 17,
        'description': 'SCRFD face detector'
    },
    'w600k_r50.onnx': {
        'url': 'https://huggingface.co/spaces/lea-ondr/insightface-demo/resolve/main/models/w600k_r50.onnx',
        'size_mb': 174,
        'description': 'ArcFace face recognition'
    }
}

def download_file(url, dest_path, expected_size_mb):
    """Download a file with progress bar."""
    print(f"Downloading {os.path.basename(dest_path)} ({expected_size_mb}MB)...")
    
    try:
        # Create SSL context that works with GitHub
        import ssl
        ssl_context = ssl.create_default_context()
        ssl_context.check_hostname = False
        ssl_context.verify_mode = ssl.CERT_NONE
        
        # Use a browser-like user agent and HF token if available
        headers = {
            'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'application/octet-stream',
        }
        
        # Add HuggingFace token if available
        hf_token = os.environ.get('HF_TOKEN') or os.environ.get('HUGGINGFACE_TOKEN')
        if hf_token:
            headers['Authorization'] = f'Bearer {hf_token}'
        
        req = urllib.request.Request(url, headers=headers)
        
        with urllib.request.urlopen(req, timeout=120, context=ssl_context) as response:
            total_size = int(response.headers.get('content-length', 0))
            downloaded = 0
            
            with open(dest_path, 'wb') as f:
                while True:
                    chunk = response.read(8192)
                    if not chunk:
                        break
                    f.write(chunk)
                    downloaded += len(chunk)
                    
                    # Show progress
                    if total_size:
                        percent = (downloaded / total_size) * 100
                        mb_downloaded = downloaded / (1024 * 1024)
                        print(f"\r  Progress: {percent:.1f}% ({mb_downloaded:.1f}MB / {total_size / (1024 * 1024):.1f}MB)", end='', flush=True)
        
        print()  # New line after progress
        
        # Verify size
        actual_size_mb = os.path.getsize(dest_path) / (1024 * 1024)
        print(f"  Downloaded: {actual_size_mb:.1f}MB")
        
        # Check if size is reasonable (at least 50% of expected)
        if actual_size_mb < expected_size_mb * 0.5:
            print(f"  ⚠️  Warning: File size ({actual_size_mb:.1f}MB) is much smaller than expected ({expected_size_mb}MB)")
            print(f"  This might be an HTML error page instead of the model file.")
            return False
            
        return True
        
    except urllib.error.HTTPError as e:
        print(f"\n  ❌ HTTP Error {e.code}: {e.reason}")
        return False
    except urllib.error.URLError as e:
        print(f"\n  ❌ Network Error: {e.reason}")
        return False
    except Exception as e:
        print(f"\n  ❌ Error: {e}")
        return False

def main():
    print("=" * 60)
    print("VibeCoding - InsightFace Model Downloader")
    print("=" * 60)
    
    # Create models directory
    os.makedirs(MODELS_DIR, exist_ok=True)
    
    success_count = 0
    for name, info in MODELS.items():
        dest_name = info.get('rename_to', name)
        dest_path = os.path.join(MODELS_DIR, dest_name)
        
        # Skip if already exists and has reasonable size
        if os.path.exists(dest_path):
            size_mb = os.path.getsize(dest_path) / (1024 * 1024)
            if size_mb >= info['size_mb'] * 0.5:
                print(f"✓ {dest_name} already exists ({size_mb:.1f}MB), skipping")
                success_count += 1
                continue
        
        if download_file(info['url'], dest_path, info['size_mb']):
            print(f"✓ {dest_name} downloaded successfully")
            success_count += 1
        else:
            print(f"✗ Failed to download {dest_name}")
            # Clean up partial download
            if os.path.exists(dest_path):
                os.remove(dest_path)
    
    print()
    print("=" * 60)
    if success_count == len(MODELS):
        print("✓ All models downloaded successfully!")
        print()
        print("Next steps:")
        print("  1. Add your photos to ./dataset/")
        print("  2. Run: go run ./cmd/main.go --serve")
        return 0
    else:
        print(f"✗ Only {success_count}/{len(MODELS)} models downloaded")
        print()
        print("Manual download instructions:")
        for name, info in MODELS.items():
            dest_name = info.get('rename_to', name)
            print(f"  {dest_name}: {info['url']}")
        return 1

if __name__ == '__main__':
    sys.exit(main())
