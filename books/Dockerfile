FROM python:3.12-alpine   
WORKDIR /app
COPY pyproject.toml ./
RUN pip install --no-cache-dir -e .
COPY . .
EXPOSE 8001
CMD ["uvicorn","main:app","--host","0.0.0.0","--port","8001"]
