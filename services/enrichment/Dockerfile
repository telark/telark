FROM python:3.14.3-alpine3.23

ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

RUN adduser -D -g "" appuser && chown -R appuser /app
USER appuser

CMD ["python", "main.py"]
